// Package gitcore runs `git -C <dir> <args...>` subprocesses for adapter packages.
// It has no domain or observability dependencies; callers own sentinel mapping and tracing.
package gitcore

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// StderrCap limits stderr included in ExecError.Error() strings.
const StderrCap = 200

// ErrGitMissing is returned when the git binary is not on PATH.
var ErrGitMissing = errors.New("gitcore: git binary not found on PATH")

// ExecError wraps a failed git subprocess with captured stderr.
type ExecError struct {
	err    error
	stderr string
}

func (e *ExecError) Error() string {
	if e.stderr == "" {
		return e.err.Error()
	}
	trimmed := e.stderr
	if len(trimmed) > StderrCap {
		trimmed = trimmed[:StderrCap] + "..."
	}
	return e.err.Error() + ": " + trimmed
}

func (e *ExecError) Unwrap() error { return e.err }

// Stderr returns the full trimmed git stderr (uncapped). Use this for classification,
// not Error(), which may truncate for log safety.
//
//funclogmeasure:skip category=hot-path reason="Pure accessor; operation trace is emitted by Run callers."
func (e *ExecError) Stderr() string { return e.stderr }

// Run invokes `git -C <dir> <args...>` and returns trimmed stdout.
//
//funclogmeasure:skip category=hot-path reason="Pure subprocess helper; operation trace is emitted by adapter callers (gitwork, harness, gitexec)."
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	dir = filepath.Clean(dir)
	all := append([]string{"-C", dir}, args...)
	cmd := exec.CommandContext(ctx, "git", all...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		if errors.Is(err, exec.ErrNotFound) {
			return "", fmt.Errorf("%w: %v", ErrGitMissing, err)
		}
		return "", &ExecError{err: err, stderr: strings.TrimSpace(stderr.String())}
	}
	return strings.TrimSpace(stdout.String()), nil
}

// Stderr extracts git stderr from err when it wraps *ExecError.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func Stderr(err error) string {
	var ee *ExecError
	if errors.As(err, &ee) {
		return ee.stderr
	}
	return ""
}

// StderrContains reports whether git stderr contains substr (case-insensitive).
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func StderrContains(err error, substr string) bool {
	return strings.Contains(strings.ToLower(Stderr(err)), strings.ToLower(substr))
}
