package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks"
)

// ListFlat returns tasks ordered by id ASC with limit/offset over
// all rows (no tree). See tasks.ListFlat for clamp rules.
func (s *Store) ListFlat(ctx context.Context, limit, offset int) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListFlat")
	return tasks.ListFlat(ctx, s.db, limit, offset)
}

// List is an alias for ListFlat. Prefer ListFlat in new code.
func (s *Store) List(ctx context.Context, limit, offset int) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.List")
	return tasks.ListFlat(ctx, s.db, limit, offset)
}

// ListRootForest pages root tasks and attaches the full descendant
// subtree per root.
func (s *Store) ListRootForest(ctx context.Context, limit, offset int) ([]TaskNode, bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListRootForest")
	return tasks.ListRootForest(ctx, s.db, limit, offset)
}

// ListRootForestAfter is the keyset-pagination variant of
// ListRootForest.
func (s *Store) ListRootForestAfter(ctx context.Context, limit int, afterID string) ([]TaskNode, bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListRootForestAfter")
	return tasks.ListRootForestAfter(ctx, s.db, limit, afterID)
}

// GetTaskTree returns one task and every descendant nested under it.
func (s *Store) GetTaskTree(ctx context.Context, id string) (TaskNode, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.GetTaskTree")
	return tasks.GetTree(ctx, s.db, id)
}
