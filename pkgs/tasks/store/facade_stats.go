package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/stats"
)

// TaskStats holds the global task counters returned by GET /tasks/stats.
// Aliased to internal/stats so the wire shape lives next to the SQL.
type TaskStats = stats.TaskStats

// TaskStats returns global counters across all tasks (totals plus by-status,
// by-priority, by-scope breakdowns).
func (s *Store) TaskStats(ctx context.Context) (TaskStats, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TaskStats")
	return stats.Get(ctx, s.db)
}
