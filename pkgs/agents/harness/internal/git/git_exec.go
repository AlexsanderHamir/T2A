package git

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
)

const logCmd = "taskapi"

// GitRepo abstracts exec-based git I/O for production and tests.
type GitRepo interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecRepo is the production GitRepo implementation.
type ExecRepo struct{}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// NewExecRepo returns an ExecRepo for harness git operations.
func NewExecRepo() *ExecRepo {
	return &ExecRepo{}
}

var defaultRepo GitRepo = NewExecRepo()

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// DefaultRepo returns the production GitRepo used when none is injected.
func DefaultRepo() GitRepo {
	return defaultRepo
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
// Run invokes git via the default production GitRepo.
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	return defaultRepo.Run(ctx, dir, args...)
}

// Run invokes `git -C <dir> <args...>` and returns trimmed stdout.
func (ExecRepo) Run(ctx context.Context, dir string, args ...string) (string, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.Run",
		"dir", dir, "args", args)
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

// IsNotAGitRepoErr distinguishes a plain directory from a git worktree.
func IsNotAGitRepoErr(err error) bool {
	slog.Debug("trace", "cmd", logCmd, "operation", "agent.harness.git.IsNotAGitRepoErr")
	var ge *execErr
	if !errors.As(err, &ge) {
		return false
	}
	s := strings.ToLower(ge.stderr)
	return strings.Contains(s, "not a git repository") ||
		strings.Contains(s, "fatal: not a git repository")
}
