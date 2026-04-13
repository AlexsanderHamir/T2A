package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ReadyTaskNotifier is invoked by the store after a task row is committed with status ready
// (on create) or when status transitions to ready (on update or dev row mirror). The store may
// hold a nil notifier (for example in tests); taskapi wires a non-nil implementation at startup.
// Implementations should avoid blocking the store caller for long (for example use a buffered channel).
type ReadyTaskNotifier interface {
	NotifyReadyTask(ctx context.Context, task domain.Task) error
}

// SetReadyTaskNotifier registers n for ready-task notifications. Pass nil to clear the notifier.
// Safe for use before serving traffic; typical wiring is once at process startup.
func (s *Store) SetReadyTaskNotifier(n ReadyTaskNotifier) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetReadyTaskNotifier", "enabled", n != nil)
	if s == nil {
		return
	}
	s.notifyMu.Lock()
	s.readyNotifier = n
	s.notifyMu.Unlock()
}

func (s *Store) notifyReadyTask(ctx context.Context, task domain.Task) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.notifyReadyTask", "task_id", task.ID)
	if s == nil || task.ID == "" {
		return
	}
	s.notifyMu.RLock()
	n := s.readyNotifier
	s.notifyMu.RUnlock()
	if n == nil {
		return
	}
	// Work is already committed; do not tie in-process delivery to request cancellation/deadlines.
	notifyCtx := context.Background()
	if ctx != nil {
		notifyCtx = context.WithoutCancel(ctx)
	}
	if err := n.NotifyReadyTask(notifyCtx, task); err != nil {
		slog.Warn("ready task notifier failed", "cmd", storeLogCmd, "operation", "tasks.store.notifyReadyTask",
			"task_id", task.ID, "err", err)
	}
}
