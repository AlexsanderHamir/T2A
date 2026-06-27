package gitwork

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"errors"
	"log/slog"
)

// ResolveRegistration opens any checkout path and returns the canonical main
// worktree root and shared git common dir used to identify the repository.
func (s *DefaultService) ResolveRegistration(ctx context.Context, path string) (mainRoot, commonDir string, err error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.ResolveRegistration")
	opened, err := s.OpenRepository(ctx, path)
	if err != nil {
		return "", "", err
	}
	mainRoot = opened.Root
	list, err := s.ListWorktrees(ctx, opened)
	if err != nil {
		return "", "", err
	}
	// git worktree list --porcelain lists the main checkout first regardless of CWD.
	if len(list) > 0 {
		mainRoot = list[0].Path
	}
	return mainRoot, opened.CommonDir, nil
}

// BelongsToRepository reports whether candidatePath shares the same git object
// database as repoRootPath (the registered main checkout).
func (s *DefaultService) BelongsToRepository(ctx context.Context, candidatePath, repoRootPath string) (bool, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.BelongsToRepository")
	candidate, err := s.OpenRepository(ctx, candidatePath)
	if err != nil {
		if errors.Is(err, ErrNotARepository) {
			return false, nil
		}
		return false, err
	}
	repo, err := s.OpenRepository(ctx, repoRootPath)
	if err != nil {
		return false, err
	}
	return sameCommonDir(candidate.CommonDir, repo.CommonDir), nil
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func sameCommonDir(a, b string) bool {
	return PathKeyEqual(a, b)
}
