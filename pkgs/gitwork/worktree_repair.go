package gitwork

import "github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
import (
	"context"
	"log/slog"
	"strings"
)

// RepairWorktrees runs git worktree repair from the main checkout root.
func (s *DefaultService) RepairWorktrees(ctx context.Context, repo *Repository) error {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.RepairWorktrees")
	if repo == nil {
		return ErrNotARepository
	}
	_, err := s.runGit(ctx, repo.Root, "worktree", "repair")
	return err
}

// PruneWorktrees runs git worktree prune from the main checkout root.
func (s *DefaultService) PruneWorktrees(ctx context.Context, repo *Repository) error {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.PruneWorktrees")
	if repo == nil {
		return ErrNotARepository
	}
	_, err := s.runGit(ctx, repo.Root, "worktree", "prune")
	return err
}

// BranchHead returns the commit SHA for refs/heads/branchName in repo.
func (s *DefaultService) BranchHead(ctx context.Context, repo *Repository, branchName string) (string, error) {
	slog.DebugContext(ctx, "trace", "cmd", calltrace.LogCmd, "operation", "gitwork.BranchHead")
	if repo == nil {
		return "", ErrNotARepository
	}
	branchName = strings.TrimSpace(branchName)
	if branchName == "" {
		return "", ErrNotARepository
	}
	out, err := s.runGit(ctx, repo.Root, "rev-parse", "refs/heads/"+branchName)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}
