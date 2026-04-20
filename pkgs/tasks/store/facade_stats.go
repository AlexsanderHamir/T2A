package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/stats"
)

// TaskStats holds the global task counters returned by GET /tasks/stats.
// Aliased to internal/stats so the wire shape lives next to the SQL.
type TaskStats = stats.TaskStats

// CycleStats / PhaseStats / RecentFailure are re-exported so handler
// JSON projection can reference them without reaching into internal/.
// RunnerStats / RunnerBucket carry the Phase 2 runner+model breakdown.
type (
	CycleStats    = stats.CycleStats
	PhaseStats    = stats.PhaseStats
	RunnerStats   = stats.RunnerStats
	RunnerBucket  = stats.RunnerBucket
	RecentFailure = stats.RecentFailure
)

// RunnerUnknownKey is the bucket key used for cycles whose meta
// predates the V2 attribution keys. Re-exported so handler tests can
// reference it without reaching into internal/.
const RunnerUnknownKey = stats.RunnerUnknownKey

// PreFeatureCycleCounts re-exports the rollout-count payload returned
// by CountPreFeatureCycles. Lives next to the other stats types so
// callers in cmd/taskapi never reach into internal/.
type PreFeatureCycleCounts = stats.PreFeatureCycleCounts

// TaskStats returns global counters across all tasks (totals plus by-status,
// by-priority, by-scope breakdowns).
func (s *Store) TaskStats(ctx context.Context) (TaskStats, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.TaskStats")
	return stats.Get(ctx, s.db)
}

// CountPreFeatureCycles returns the count of terminal task_cycles whose
// meta_json predates the V2 runner/model attribution keys. Intended for
// the agent worker supervisor's one-shot startup log line so operators
// know how much cycle history is unattributed; not wired into any
// hot path. See pkgs/tasks/store/internal/stats/count_pre_feature.go
// for the per-bucket semantics.
func (s *Store) CountPreFeatureCycles(ctx context.Context) (PreFeatureCycleCounts, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.CountPreFeatureCycles")
	return stats.CountPreFeatureCycles(ctx, s.db)
}
