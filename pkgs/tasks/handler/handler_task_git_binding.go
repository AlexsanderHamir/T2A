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
func trimmedOptionalID(id *string) string {
	if id == nil {
		return ""
	}
	return strings.TrimSpace(*id)
}

func (h *Handler) validateTaskGitBindingV2(
	ctx context.Context,
	projectID *string,
	worktreeID *string,
) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validateTaskGitBindingV2")
	wtID := trimmedOptionalID(worktreeID)
	if wtID == "" {
		return nil
	}
	return h.store.ValidateTaskWorktreeBinding(ctx, projectID, wtID)
}

func (h *Handler) validatePromptMentionsForWorktree(ctx context.Context, worktreeID *string, prompt string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validatePromptMentionsForWorktree")
	wtID := trimmedOptionalID(worktreeID)
	if wtID == "" {
		if len(repo.ParseFileMentions(prompt)) > 0 {
			return fmt.Errorf("%w: worktree_id required for @-mentions", domain.ErrInvalidInput)
		}
		return nil
	}
	return h.validatePromptMentionsForWorktreeID(ctx, wtID, prompt)
}

func (h *Handler) validatePromptMentionsForWorktreeID(ctx context.Context, worktreeID, prompt string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.validatePromptMentionsForWorktreeID")
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
