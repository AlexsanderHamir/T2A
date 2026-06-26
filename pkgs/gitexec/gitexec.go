// Package gitexec runs bounded, fixed-subcommand git operations for HTTP and
// other non-harness callers. It does not expose arbitrary git argument passthrough.
package gitexec

import (
	"context"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitcore"
)

// DefaultMaxPatchBytes caps commit patch payloads returned over HTTP.
const DefaultMaxPatchBytes = 512 * 1024

// Run invokes `git -C <dir> <args...>` and returns trimmed stdout.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	return gitcore.Run(ctx, dir, args...)
}

// ShowCommitPatch returns the unified diff for one commit (parent..commit).
// maxBytes limits the returned patch size; truncated is true when the cap applies.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
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

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func isNotFoundErr(err error) bool {
	return gitcore.StderrContains(err, "bad object") ||
		gitcore.StderrContains(err, "unknown revision") ||
		gitcore.StderrContains(err, "not a valid object name") ||
		gitcore.StderrContains(err, "ambiguous argument") ||
		gitcore.StderrContains(err, "not a git repository")
}
