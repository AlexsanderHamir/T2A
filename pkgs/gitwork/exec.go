package gitwork

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"strings"
	"time"

	"github.com/AlexsanderHamir/Hamix/pkgs/gitcore"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

func (s *DefaultService) runGit(ctx context.Context, dir string, args ...string) (string, error) {
	start := time.Now()
	dir = filepath.Clean(dir)
	out, err := gitcore.Run(ctx, dir, args...)
	slog.DebugContext(ctx, "git command",
		"cmd", calltrace.LogCmd,
		"operation", "gitwork.runGit",
		"dir", dir,
		"args", args,
		"duration_ms", time.Since(start).Milliseconds(),
	)
	if err == nil {
		return out, nil
	}
	if errors.Is(err, gitcore.ErrGitMissing) {
		return "", fmt.Errorf("%w: %v", ErrGitMissing, err)
	}
	return "", err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func stderrContains(err error, substr string) bool {
	return gitcore.StderrContains(err, substr)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapNotARepository(err error) error {
	if err == nil {
		return nil
	}
	if stderrContains(err, "not a git repository") {
		return ErrNotARepository
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapWorktreeAddErr(err error) error {
	if err == nil {
		return nil
	}
	if stderrContains(err, "is already checked out") ||
		stderrContains(err, "already used by worktree") {
		return ErrBranchCheckedOut
	}
	if stderrContains(err, "already exists") {
		return ErrWorktreeExists
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapBranchCreateErr(err error) error {
	if err == nil {
		return nil
	}
	if stderrContains(err, "already exists") {
		return ErrBranchExists
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapBranchDeleteErr(err error) error {
	if err == nil {
		return nil
	}
	if stderrContains(err, "checked out") ||
		stderrContains(err, "used by worktree") {
		return ErrBranchCheckedOut
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapWorktreeRemoveErr(err error) error {
	if err == nil {
		return nil
	}
	if stderrContains(err, "modified or untracked files") ||
		strings.Contains(strings.ToLower(gitcore.Stderr(err)), "contains modified") {
		return ErrDirty
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func mapCheckoutErr(err error) error {
	if err == nil {
		return nil
	}
	if stderrContains(err, "is already checked out") ||
		stderrContains(err, "used by worktree") {
		return ErrBranchCheckedOut
	}
	if stderrContains(err, "local changes") ||
		stderrContains(err, "would be overwritten") {
		return ErrDirty
	}
	return err
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func absPath(p string) (string, error) {
	p = filepath.Clean(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(abs), nil
}
