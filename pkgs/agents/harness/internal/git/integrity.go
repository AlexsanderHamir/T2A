package git

import (
	"context"
	"log/slog"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// VerifyTamperedReason is the cycle terminate reason when integrity check fails.
const VerifyTamperedReason = "verify_tampered"

// VerifyIntegrityCheckTimeoutReason is recorded when the post-snapshot cannot complete.
const VerifyIntegrityCheckTimeoutReason = "verify_integrity_check_timeout"

const integritySnapshotTimeout = 30 * time.Second

// IntegritySnapshot captures version-controlled state for verify bracketing.
type IntegritySnapshot struct {
	NotGitRepo bool
	Head       string
	Changed    map[string]struct{}
}

// IntegrityDiff reports paths added between pre and post snapshots.
type IntegrityDiff struct {
	HeadChanged bool
	AddedPaths  []string
}

// CaptureIntegritySnapshot runs git rev-parse HEAD and git status --porcelain -z.
func CaptureIntegritySnapshot(ctx context.Context, repo GitRepo, workingDir string) (IntegritySnapshot, error) {
	if repo == nil {
		repo = DefaultRepo()
	}
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.CaptureIntegritySnapshot",
		"working_dir", workingDir)

	probeCtx, cancel := context.WithTimeout(ctx, integritySnapshotTimeout)
	defer cancel()

	head, err := repo.Run(probeCtx, workingDir, "rev-parse", "HEAD")
	if err != nil {
		if IsNotAGitRepoErr(err) {
			return IntegritySnapshot{NotGitRepo: true}, nil
		}
		return IntegritySnapshot{}, err
	}
	statusOut, err := repo.Run(probeCtx, workingDir, "status", "--porcelain", "-z")
	if err != nil {
		if IsNotAGitRepoErr(err) {
			return IntegritySnapshot{NotGitRepo: true}, nil
		}
		return IntegritySnapshot{}, err
	}
	changed := parsePorcelainZ(statusOut)
	return IntegritySnapshot{Head: strings.TrimSpace(head), Changed: changed}, nil
}

// DiffIntegritySnapshots reports paths that appear in post but not in pre.
func DiffIntegritySnapshots(pre, post IntegritySnapshot) IntegrityDiff {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.DiffIntegritySnapshots")
	if pre.NotGitRepo || post.NotGitRepo {
		return IntegrityDiff{}
	}
	d := IntegrityDiff{HeadChanged: pre.Head != post.Head}
	for p := range post.Changed {
		if _, ok := pre.Changed[p]; !ok {
			d.AddedPaths = append(d.AddedPaths, p)
		}
	}
	sort.Strings(d.AddedPaths)
	return d
}

// ClassifyIntegrityDiff turns a diff into a tampered flag and operator summary.
func ClassifyIntegrityDiff(diff IntegrityDiff, cycleID string) (bool, string) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.ClassifyIntegrityDiff",
		"cycle_id", cycleID, "head_changed", diff.HeadChanged, "added_count", len(diff.AddedPaths))
	if diff.HeadChanged {
		return true, "HEAD ref moved during verify pass"
	}
	if len(diff.AddedPaths) == 0 {
		return false, ""
	}
	return true, summariseTamperedPaths(diff.AddedPaths)
}

// CheckVerifyIntegrity performs the post-verify integrity check.
func CheckVerifyIntegrity(ctx context.Context, repo GitRepo, workingDir, cycleID string, pre IntegritySnapshot, preErr error) (bool, string) {
	if repo == nil {
		repo = DefaultRepo()
	}
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.CheckVerifyIntegrity",
		"cycle_id", cycleID)
	if pre.NotGitRepo {
		return false, ""
	}
	if preErr != nil {
		return true, "pre-verify integrity snapshot failed: " + preErr.Error()
	}
	post, err := CaptureIntegritySnapshot(ctx, repo, workingDir)
	if err != nil {
		slog.Warn("agent harness post-verify integrity snapshot failed",
			"cmd", logCmd, "operation", "agent.harness.git.CheckVerifyIntegrity.post_snapshot_err",
			"cycle_id", cycleID, "err", err)
		return true, "post-verify integrity snapshot failed: " + err.Error()
	}
	if post.NotGitRepo {
		return true, ".git directory disappeared during verify pass"
	}
	diff := DiffIntegritySnapshots(pre, post)
	tampered, summary := ClassifyIntegrityDiff(diff, cycleID)
	if tampered {
		slog.Warn("verify pass tampered with working dir",
			"cmd", logCmd, "operation", "agent.harness.git.CheckVerifyIntegrity.tampered",
			"cycle_id", cycleID, "summary", summary)
	}
	return tampered, summary
}

func summariseTamperedPaths(paths []string) string {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.summariseTamperedPaths",
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

func parsePorcelainZ(out string) map[string]struct{} {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.parsePorcelainZ",
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

// WorkingTreeDirty reports whether the worktree has uncommitted changes.
func WorkingTreeDirty(ctx context.Context, repo GitRepo, worktree string) (bool, error) {
	snap, err := CaptureIntegritySnapshot(ctx, repo, worktree)
	if err != nil {
		return false, err
	}
	if snap.NotGitRepo {
		return false, nil
	}
	return len(snap.Changed) > 0, nil
}

// StatusPorcelain returns trimmed porcelain status, truncated for prompts.
func StatusPorcelain(ctx context.Context, repo GitRepo, workdir string) (string, error) {
	if repo == nil {
		repo = DefaultRepo()
	}
	out, err := repo.Run(ctx, workdir, "status", "--porcelain")
	if err != nil {
		return "", err
	}
	const maxLen = 2048
	out = strings.TrimSpace(out)
	if len(out) > maxLen {
		out = out[:maxLen] + "\n…"
	}
	return out, nil
}

// ScopeFilesFromPhaseDetails lists paths changed between cycle base and HEAD.
func ScopeFilesFromPhaseDetails(ctx context.Context, repo GitRepo, workdir string, details []byte) []string {
	if repo == nil {
		repo = DefaultRepo()
	}
	if len(details) == 0 {
		return nil
	}
	base := CycleBaseFromPhaseDetails(details)
	if base == "" {
		return nil
	}
	out, err := repo.Run(ctx, workdir, "diff", "--name-only", base+"..HEAD")
	if err != nil {
		return nil
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	var files []string
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			files = append(files, line)
		}
	}
	return files
}

// Itoa is exported for tests that assert truncated integrity summaries.
func Itoa(n int) string { return itoa(n) }
