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
func trimmedOptionalID(id *string) string {
	if id == nil {
		return ""
	}
	return strings.TrimSpace(*id)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func trimmedWorktreeID(worktreeID *string) string {
	return trimmedOptionalID(worktreeID)
}

func (h *Handler) validateTaskGitBinding(
	ctx context.Context,
	projectID *string,
	worktreeID, branchID, worktreeBranchID *string,
) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validateTaskGitBinding")
	wbID := trimmedOptionalID(worktreeBranchID)
	if wbID != "" {
		if err := h.validateLegacyGitPairPartial(worktreeID, branchID); err != nil {
			return err
		}
		if err := h.store.ValidateTaskWorktreeBranchBinding(ctx, projectID, wbID); err != nil {
			return err
		}
		assoc, err := h.store.GetWorktreeBranchByID(ctx, wbID)
		if err != nil {
			return err
		}
		return h.store.GuardBranchNotActiveElsewhere(ctx, assoc.WorktreeID, assoc.BranchID)
	}
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
	return h.store.GuardBranchNotActiveElsewhere(ctx, wt, br)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (h *Handler) validateLegacyGitPairPartial(worktreeID, branchID *string) error {
	wt := trimmedOptionalID(worktreeID)
	br := trimmedOptionalID(branchID)
	if wt == "" && br == "" {
		return nil
	}
	if wt == "" || br == "" {
		return fmt.Errorf("%w: worktree_id and branch_id must both be set when worktree_branch_id is omitted", domain.ErrInvalidInput)
	}
	return nil
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
