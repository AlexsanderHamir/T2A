package git

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitcore"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

// GitRepo abstracts exec-based git I/O for production and tests.
type GitRepo interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// ExecRepo is the production GitRepo implementation.
type ExecRepo struct{}

// NewExecRepo returns an ExecRepo for harness git operations.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func NewExecRepo() *ExecRepo {
	return &ExecRepo{}
}

var defaultRepo GitRepo = NewExecRepo()

// DefaultRepo returns the production GitRepo used when none is injected.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func DefaultRepo() GitRepo {
	return defaultRepo
}

// Run invokes git via the default production GitRepo.
//
//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func Run(ctx context.Context, dir string, args ...string) (string, error) {
	return defaultRepo.Run(ctx, dir, args...)
}

// Run invokes `git -C <dir> <args...>` and returns trimmed stdout.
func (ExecRepo) Run(ctx context.Context, dir string, args ...string) (string, error) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.harness.git.Run",
		"dir", dir, "args", args)
	return gitcore.Run(ctx, dir, args...)
}

// IsNotAGitRepoErr distinguishes a plain directory from a git worktree.
func IsNotAGitRepoErr(err error) bool {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "agent.harness.git.IsNotAGitRepoErr")
	return gitcore.StderrContains(err, "not a git repository") ||
		gitcore.StderrContains(err, "fatal: not a git repository")
}
