package harness

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// verify_integrity.go is the post-verify safety net. The verify pass
// runs in execute's working directory because that is where execute's
// uncommitted changes live and where the verifier can actually inspect
// file contents. Trade-off: nothing structurally prevents a misbehaving
// verifier from mutating source. So we bracket the verify work with a
// pre/post snapshot of the working dir and treat any mutation outside
// the verifier's documented output file as tampering.
//
// What we capture:
//
//   - `git rev-parse HEAD` — catches ref/.git tampering. `git status`
//     does not see those, so without this we would miss a verifier
//     that ran `git update-ref` or corrupted the index/object DB.
//   - `git status --porcelain` — every working-tree change relative
//     to HEAD, both staged and unstaged. The diff between pre and post
//     is what we audit.
//
// Stance under uncertainty: if the post-snapshot itself errors (e.g.
// verify corrupted .git/), we treat it as tampering. A safety property
// cannot be defeated by "the check threw an exception". Likewise if
// HEAD is unstable across the bracket, we fail closed regardless of
// what the working-tree diff would have said.
//
// When the working dir is not a git repo (test fixtures, spike work):
// both calls return "not a git repo" and we degrade to a no-op so the
// rest of the verification pipeline still runs. The supervisor logs
// the missing safety net once at startup; we do not log per-cycle.

// verifyTamperedReason is the cycle terminate reason recorded when the
// integrity check finds source mutations after the verify pass. Pinned
// here so the audit trail string is stable across refactors.
const verifyTamperedReason = "verify_tampered"

// verifyIntegrityCheckTimeoutReason is recorded when the post-snapshot
// itself cannot complete within the integrity probe budget. Distinct
// from verifyTamperedReason so post-mortems can tell "verifier
// modified source" from "git status hung on a huge repo".
const verifyIntegrityCheckTimeoutReason = "verify_integrity_check_timeout"

// integritySnapshotTimeout bounds each git invocation. Bigger than the
// largest reasonable git-status latency; short enough that an actually
// hung git keeps the worker responsive. The deterministic-check
// timeout is configurable per-install; this one is intentionally not,
// because it gates a safety property and operators should not be able
// to disable it by setting it to zero.
const integritySnapshotTimeout = 30 * time.Second

// integritySnapshot is the captured-once view of the working dir's
// version-controlled state. The ID set is "every path git status
// reports as changed", normalised; HEAD is the resolved commit hash.
//
// notGitRepo=true means the working dir is not a git repo and the
// integrity check is bypassed for this cycle. The rest of the fields
// are zero in that case.
type integritySnapshot struct {
	notGitRepo bool
	head       string
	changed    map[string]struct{}
}

// captureIntegritySnapshot runs `git rev-parse HEAD` and
// `git status --porcelain -z` against workingDir. The -z form uses NUL
// separators so paths with spaces or quotes are unambiguous; the
// porcelain v1 format puts a 2-char status + space + path on each
// record, which we strip before normalising.
func captureIntegritySnapshot(ctx context.Context, workingDir string) (integritySnapshot, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.captureIntegritySnapshot",
		"working_dir", workingDir)

	probeCtx, cancel := context.WithTimeout(ctx, integritySnapshotTimeout)
	defer cancel()

	head, err := runGit(probeCtx, workingDir, "rev-parse", "HEAD")
	if err != nil {
		if isNotAGitRepoErr(err) {
			return integritySnapshot{notGitRepo: true}, nil
		}
		return integritySnapshot{}, err
	}
	statusOut, err := runGit(probeCtx, workingDir, "status", "--porcelain", "-z")
	if err != nil {
		if isNotAGitRepoErr(err) {
			return integritySnapshot{notGitRepo: true}, nil
		}
		return integritySnapshot{}, err
	}
	changed := parsePorcelainZ(statusOut)
	return integritySnapshot{head: strings.TrimSpace(head), changed: changed}, nil
}

// diffIntegritySnapshots reports paths that appear in post but not in
// pre, plus a flag for HEAD instability. The intent: the operator
// should see exactly which paths the verifier touched, so the cycle
// failure summary can carry actionable detail without being unbounded.
//
// We compare HEAD strings directly; if they differ the verifier ran
// git plumbing that moved refs and the integrity check fails closed
// regardless of working-tree state.
type integrityDiff struct {
	headChanged bool
	addedPaths  []string
}

func diffIntegritySnapshots(pre, post integritySnapshot) integrityDiff {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.diffIntegritySnapshots")
	if pre.notGitRepo || post.notGitRepo {
		return integrityDiff{}
	}
	d := integrityDiff{headChanged: pre.head != post.head}
	for p := range post.changed {
		if _, ok := pre.changed[p]; !ok {
			d.addedPaths = append(d.addedPaths, p)
		}
	}
	sort.Strings(d.addedPaths)
	return d
}

// classifyIntegrityDiff turns the diff into a (tampered, summary)
// pair. summary is a short, operator-facing string suitable for the
// cycle's terminate_reason field; it lists at most a handful of paths
// so the SPA cycle-list view does not need to truncate. If tampered
// is false the summary is "".
//
// The whitelist is empty by design: the worker writes report files to
// Options.ReportDir (outside RepoRoot) so the verifier cannot legally
// modify anything inside the working tree during the verify pass. Any
// added path here is tampering, full stop. This is a tightening of
// the previous .t2a/<cycleID>/verify-report.json allowance — files no
// longer live under RepoRoot and therefore cannot show up in the
// porcelain diff under any non-misbehaving codepath.
func classifyIntegrityDiff(diff integrityDiff, cycleID string) (bool, string) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.classifyIntegrityDiff",
		"cycle_id", cycleID, "head_changed", diff.headChanged, "added_count", len(diff.addedPaths))
	if diff.headChanged {
		return true, "HEAD ref moved during verify pass"
	}
	if len(diff.addedPaths) == 0 {
		return false, ""
	}
	return true, summariseTamperedPaths(diff.addedPaths)
}

// summariseTamperedPaths renders up to 5 paths inline; anything beyond
// gets a "+N more" tail so the operator knows the violation set is
// larger than the displayed slice.
func summariseTamperedPaths(paths []string) string {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.summariseTamperedPaths",
		"path_count", len(paths))
	const maxInline = 5
	if len(paths) <= maxInline {
		return "verifier modified " + strings.Join(paths, ",")
	}
	head := strings.Join(paths[:maxInline], ",")
	rest := len(paths) - maxInline
	return "verifier modified " + head + " (+" + itoa(rest) + " more)"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	digits := []byte{}
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	if neg {
		digits = append([]byte{'-'}, digits...)
	}
	return string(digits)
}

// runGit invokes `git -C <dir> <args...>` and returns trimmed stdout.
// Stderr is captured into the error so callers can detect "not a git
// repo" and degrade to bypass.
func runGit(ctx context.Context, dir string, args ...string) (string, error) {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.runGit",
		"dir", dir, "args", args)
	dir = filepath.Clean(dir)
	all := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", all...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &gitErr{err: err, stderr: strings.TrimSpace(stderr.String())}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// gitErr wraps an exec error with stderr text for sniffing.
type gitErr struct {
	err    error
	stderr string
}

func (e *gitErr) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + e.stderr
}

func (e *gitErr) Unwrap() error { return e.err }

// isNotAGitRepoErr distinguishes "operator pointed RepoRoot at a plain
// directory" from real git failures (timeout, permission denied, etc).
// The stderr fragment is stable across modern git versions; falling
// back to substring match keeps us off the brittle exit-code path.
func isNotAGitRepoErr(err error) bool {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.isNotAGitRepoErr")
	var ge *gitErr
	if !errors.As(err, &ge) {
		return false
	}
	s := strings.ToLower(ge.stderr)
	return strings.Contains(s, "not a git repository") ||
		strings.Contains(s, "fatal: not a git repository")
}

// parsePorcelainZ parses `git status --porcelain -z` output into a set
// of changed path strings (forward-slash, no XY prefix). The format
// per record is "XY path\0", with rename records using "XY orig\0
// renamed\0" — we keep the renamed path and discard the original
// (which would otherwise show up as "removed" pre-verify if execute
// staged the rename, polluting the diff).
func parsePorcelainZ(out string) map[string]struct{} {
	slog.Debug("trace", "cmd", harnessLogCmd, "operation", "agent.harness.parsePorcelainZ",
		"len", len(out))
	result := map[string]struct{}{}
	if out == "" {
		return result
	}
	records := strings.Split(out, "\x00")
	i := 0
	for i < len(records) {
		rec := records[i]
		if len(rec) < 3 {
			i++
			continue
		}
		xy := rec[:2]
		path := strings.TrimSpace(rec[3:])
		if path == "" {
			i++
			continue
		}
		if xy[0] == 'R' || xy[1] == 'R' {
			i++
			if i < len(records) {
				renamed := strings.TrimSpace(records[i])
				if renamed != "" {
					result[filepath.ToSlash(renamed)] = struct{}{}
				}
			}
			i++
			continue
		}
		result[filepath.ToSlash(path)] = struct{}{}
		i++
	}
	return result
}
