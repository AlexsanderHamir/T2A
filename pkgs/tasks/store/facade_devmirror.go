package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/devmirror"
)

// ApplyDevTaskRowMirror updates the task row to reflect a synthetic
// audit event without appending further audit rows. For development
// simulation only (see pkgs/tasks/devsim). Fires the ready-task
// notifier when the row transitions into StatusReady.
func (s *Store) ApplyDevTaskRowMirror(ctx context.Context, taskID string, typ domain.EventType, data []byte) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApplyDevTaskRowMirror")
	nt, prev, err := devmirror.ApplyTaskRowMirror(ctx, s.db, taskID, typ, data)
	if err != nil {
		return err
	}
	if nt.Status == domain.StatusReady && prev != domain.StatusReady {
		s.notifyReadyTask(ctx, *nt)
	}
	return nil
}

// ListDevsimTasks returns tasks whose id matches a SQL LIKE pattern
// (dev simulation only).
func (s *Store) ListDevsimTasks(ctx context.Context, idLikePattern string) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDevsimTasks")
	return devmirror.ListDevsimTasks(ctx, s.db, idLikePattern)
}
