package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/commits"
)

// CycleCommitEntry is the public re-export of a commit upsert payload.
type CycleCommitEntry = commits.Entry

// UpsertCycleCommits persists worker-indexed git commits for one cycle.
func (s *Store) UpsertCycleCommits(ctx context.Context, taskID, cycleID string, entries []CycleCommitEntry) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.UpsertCycleCommits")
	return commits.UpsertCycleCommits(ctx, s.db, taskID, cycleID, entries)
}

// ListCommitsForCycle returns commits for a cycle ordered by ancestry seq.
func (s *Store) ListCommitsForCycle(ctx context.Context, cycleID string) ([]domain.TaskCycleCommit, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListCommitsForCycle")
	return commits.ListCommitsForCycle(ctx, s.db, cycleID)
}

// ListCommitsForTask returns distinct commits indexed for a task across all cycles.
func (s *Store) ListCommitsForTask(ctx context.Context, taskID string) ([]domain.TaskCycleCommit, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListCommitsForTask")
	return commits.ListCommitsForTask(ctx, s.db, taskID)
}
