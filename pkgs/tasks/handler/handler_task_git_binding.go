package handler

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/Hamix/pkgs/repo"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
)

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func taskHasGitBinding(worktreeID, branchID *string) bool {
	if worktreeID == nil || branchID == nil {
		return false
	}
	return strings.TrimSpace(*worktreeID) != "" && strings.TrimSpace(*branchID) != ""
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func trimmedWorktreeID(worktreeID *string) string {
	if worktreeID == nil {
		return ""
	}
	return strings.TrimSpace(*worktreeID)
}

func (h *Handler) validateTaskGitBinding(ctx context.Context, projectID *string, worktreeID, branchID *string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validateTaskGitBinding")
	if !taskHasGitBinding(worktreeID, branchID) {
		if worktreeID != nil && strings.TrimSpace(*worktreeID) != "" {
			return fmt.Errorf("%w: branch_id required when worktree_id is set", domain.ErrInvalidInput)
		}
		if branchID != nil && strings.TrimSpace(*branchID) != "" {
			return fmt.Errorf("%w: worktree_id required when branch_id is set", domain.ErrInvalidInput)
		}
		return nil
	}
	wt := strings.TrimSpace(*worktreeID)
	br := strings.TrimSpace(*branchID)
	if err := h.store.ValidateTaskGitBinding(ctx, projectID, wt, br); err != nil {
		return err
	}
	h.warnBranchCheckedOutElsewhere(ctx, wt, br)
	return nil
}

func (h *Handler) warnBranchCheckedOutElsewhere(ctx context.Context, worktreeID, branchID string) {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.warnBranchCheckedOutElsewhere")
	if h.git == nil {
		return
	}
	wt, err := h.store.GetGitWorktreeByID(ctx, worktreeID)
	if err != nil {
		return
	}
	br, err := h.store.GetGitBranchByID(ctx, branchID)
	if err != nil {
		return
	}
	repoRow, err := h.store.GetGitRepositoryByID(ctx, wt.RepositoryID)
	if err != nil {
		return
	}
	opened, err := h.git.OpenRepository(ctx, repoRow.Path)
	if err != nil {
		return
	}
	worktrees, err := h.git.ListWorktrees(ctx, opened)
	if err != nil {
		return
	}
	for _, other := range worktrees {
		if other.Path == wt.Path {
			continue
		}
		if other.Branch == br.Name {
			slog.Warn("branch checked out in another worktree at task save",
				"cmd", calltrace.LogCmd, "operation", "handler.task_git_binding.branch_elsewhere",
				"worktree_id", worktreeID, "branch_id", branchID, "other_worktree", other.Path)
			return
		}
	}
}

func (h *Handler) validatePromptMentionsForWorktree(ctx context.Context, worktreeID, prompt string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validatePromptMentionsForWorktree")
	if h.repoProv == nil {
		return nil
	}
	worktreeID = strings.TrimSpace(worktreeID)
	if worktreeID == "" {
		if len(repo.ParseFileMentions(prompt)) > 0 {
			return fmt.Errorf("%w: worktree_id required for @-mentions", domain.ErrInvalidInput)
		}
		return nil
	}
	root, reason, err := h.repoProv.OpenWorktreeRoot(ctx, worktreeID)
	if err != nil {
		return err
	}
	if root == nil {
		if reason == RepoReasonWorktreeNotFound {
			return fmt.Errorf("%w: worktree not found", domain.ErrNotFound)
		}
		return fmt.Errorf("%w: %s", domain.ErrInvalidInput, reason)
	}
	return root.ValidatePromptMentions(prompt)
}

// validatePromptMentionsIfRepo validates mentions against worktree_id when provided.
func (h *Handler) validatePromptMentionsIfRepo(ctx context.Context, worktreeID *string, prompt string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validatePromptMentionsIfRepo")
	wt := trimmedWorktreeID(worktreeID)
	if wt != "" {
		return h.validatePromptMentionsForWorktree(ctx, wt, prompt)
	}
	if len(repo.ParseFileMentions(prompt)) > 0 {
		return fmt.Errorf("%w: worktree_id required for @-mentions", domain.ErrInvalidInput)
	}
	return nil
}
