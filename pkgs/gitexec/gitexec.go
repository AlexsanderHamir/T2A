// Package gitexec runs bounded, fixed-subcommand git operations for HTTP and
// other non-harness callers. It does not expose arbitrary git argument passthrough.
package gitexec

import (
	"bytes"
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
)

// DefaultMaxPatchBytes caps commit patch payloads returned over HTTP.
const DefaultMaxPatchBytes = 512 * 1024

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// Run invokes `git -C <dir> <args...>` and returns trimmed stdout.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	dir = filepath.Clean(dir)
	all := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", all...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &execErr{err: err, stderr: strings.TrimSpace(stderr.String())}
	}
	return strings.TrimSpace(stdout.String()), nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// ShowCommitPatch returns the unified diff for one commit (parent..commit).
// maxBytes limits the returned patch size; truncated is true when the cap applies.
func ShowCommitPatch(ctx context.Context, dir, sha string, maxBytes int) (patch string, truncated bool, err error) {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxPatchBytes
	}
	if _, err := Run(ctx, dir, "cat-file", "-e", sha+"^{commit}"); err != nil {
		if isNotFoundErr(err) {
			return "", false, ErrNotFound
		}
		return "", false, err
	}
	out, err := Run(ctx, dir, "show", sha, "--format=", "--no-color", "--no-ext-diff")
	if err != nil {
		if isNotFoundErr(err) {
			return "", false, ErrNotFound
		}
		return "", false, err
	}
	if len(out) > maxBytes {
		return out[:maxBytes], true, nil
	}
	return out, false, nil
}

type execErr struct {
	err    error
	stderr string
}

func (e *execErr) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	return e.err.Error() + ": " + e.stderr
}

func (e *execErr) Unwrap() error { return e.err }

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func isNotFoundErr(err error) bool {
	var ge *execErr
	if !errors.As(err, &ge) {
		return false
	}
	s := strings.ToLower(ge.stderr)
	return strings.Contains(s, "bad object") ||
		strings.Contains(s, "unknown revision") ||
		strings.Contains(s, "not a valid object name") ||
		strings.Contains(s, "ambiguous argument") ||
		strings.Contains(s, "not a git repository")
}
