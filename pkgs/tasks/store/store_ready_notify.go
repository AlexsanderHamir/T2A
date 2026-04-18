package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/notify"
)

// ReadyTaskNotifier is invoked by the store after a task row is committed with status ready
// (on create) or when status transitions to ready (on update or dev row mirror). The store may
// hold a nil notifier (for example in tests); taskapi wires a non-nil implementation at startup.
// Implementations should avoid blocking the store caller for long (for example use a buffered channel).
//
// The interface lives in pkgs/tasks/store/internal/notify so subpackages can publish without
// taking a dependency on the public facade. The alias here keeps existing callers
// (cmd/taskapi/run_helpers.go, tests) compiling unchanged.
type ReadyTaskNotifier = notify.Notifier

// SetReadyTaskNotifier registers n for ready-task notifications. Pass nil to clear the notifier.
// Safe for use before serving traffic; typical wiring is once at process startup.
func (s *Store) SetReadyTaskNotifier(n ReadyTaskNotifier) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetReadyTaskNotifier", "enabled", n != nil)
	if s == nil {
		return
	}
	s.notify.Set(n)
}

// notifyReadyTask is the package-internal entrypoint used by CRUD,
// update, and dev-mirror code paths. It forwards to the holder so the
// concurrency policy lives in one place.
func (s *Store) notifyReadyTask(ctx context.Context, task domain.Task) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.notifyReadyTask", "task_id", task.ID)
	if s == nil {
		return
	}
	s.notify.Notify(ctx, task)
}
