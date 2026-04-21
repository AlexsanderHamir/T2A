package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/ready"
)

// ReadyTaskQueueCursor is a keyset cursor for ListReadyTaskQueueCandidates.
// pkgs/agents/reconcile.go is a documented caller and reads the named fields
// directly. Aliased to internal/ready so additions stay in one place.
type ReadyTaskQueueCursor = ready.QueueCursor

// ReadyTaskQueueCandidate is one ready task plus scheduling metadata for the
// agent queue (see pkgs/agents/reconcile.go).
type ReadyTaskQueueCandidate = ready.QueueCandidate

// DeferredPickup is a ready task with pickup_not_before still in the future.
type DeferredPickup = ready.DeferredPickup

// ListDeferredReadyPickupTasks returns ready tasks deferred by pickup_not_before
// after now, ordered by pickup time (for pickup wake hydration).
func (s *Store) ListDeferredReadyPickupTasks(ctx context.Context, limit int) ([]DeferredPickup, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListDeferredReadyPickupTasks")
	return ready.ListDeferredReadyPickups(ctx, s.db, time.Now().UTC(), limit)
}

// ListReadyTaskQueueCandidates returns ready tasks ordered for fair scheduling
// (see internal/ready). Pagination is keyset; pass the cursor from the last row.
func (s *Store) ListReadyTaskQueueCandidates(ctx context.Context, limit int, cursor *ReadyTaskQueueCursor) ([]ReadyTaskQueueCandidate, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListReadyTaskQueueCandidates")
	return ready.ListQueueCandidates(ctx, s.db, limit, cursor)
}

// ListReadyTasksUserCreated returns tasks with status ready whose first audit
// row is task_created by user (the user-task agent queue policy).
func (s *Store) ListReadyTasksUserCreated(ctx context.Context, limit int, afterID string) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListReadyTasksUserCreated")
	return ready.ListUserCreated(ctx, s.db, limit, afterID)
}
