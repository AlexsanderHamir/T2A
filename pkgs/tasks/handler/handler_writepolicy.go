package handler

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/calltrace"
)

// notifyTaskUpdatedEnriched loads the post-commit task row and publishes an
// enriched task_updated frame. Call only after the store mutation succeeds
// (ADR-0026 invariant S1–S2).
func (h *Handler) notifyTaskUpdatedEnriched(ctx context.Context, taskID string) error {
	slog.Debug("trace", "cmd", calltrace.LogCmd, "operation", "handler.Handler.notifyTaskUpdatedEnriched")
	task, err := h.store.Get(ctx, taskID)
	if err != nil {
		return fmt.Errorf("notify task_updated enriched: %w", err)
	}
	h.notifyTaskChanged(TaskUpdated, taskID, task)
	return nil
}
