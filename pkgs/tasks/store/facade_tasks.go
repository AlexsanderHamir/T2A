package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks"
)

// ShouldNotifyReadyNow returns true when a freshly-ready task should
// be pushed to the in-memory ready queue immediately. The invariant
// the in-memory queue MUST satisfy is that it never contains a task
// the SQL filter (ready.ListQueueCandidates) would reject; the SQL
// filter excludes any task whose pickup_not_before is still in the
// future. Without this gate, a brand-new ready task with an explicit
// future pickup time would race the reconcile loop and be picked up
// immediately by the worker — see docs/SCHEDULING.md ("the two queues").
//
// `now` is injected so tests can pin the comparison; production callers
// pass time.Now().UTC().
//
// `pickupNotBefore` is the task's pickup_not_before column value
// (nil = no deferral; in the past = effectively no deferral).
func ShouldNotifyReadyNow(pickupNotBefore *time.Time, now time.Time) bool {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ShouldNotifyReadyNow",
		"has_pickup", pickupNotBefore != nil)
	if pickupNotBefore == nil {
		return true
	}
	// strict After: a pickup time exactly equal to `now` is treated
	// as "ready" — the SQL filter uses `<= now()` and we mirror it.
	return !pickupNotBefore.After(now)
}

// CreateTaskInput is the public re-export of the task creation
// payload. The alias keeps every existing call-site unchanged while
// the implementation lives in internal/tasks.
type CreateTaskInput = tasks.CreateInput

// UpdateTaskInput is the public re-export of the task patch payload.
type UpdateTaskInput = tasks.UpdateInput

// ParentFieldPatch is the public re-export of the parent-id patch
// helper used by UpdateTaskInput.Parent.
type ParentFieldPatch = tasks.ParentFieldPatch

// PickupNotBeforePatch is the public re-export of the
// pickup_not_before patch helper used by UpdateTaskInput.PickupNotBefore.
// See docs/SCHEDULING.md.
type PickupNotBeforePatch = tasks.PickupNotBeforePatch

// TaskNode is a task row plus nested children for API tree
// responses. Re-exported from internal/tasks.
type TaskNode = tasks.Node

// MaxTaskTreeDepth bounds the nesting depth for tree responses. It
// must stay aligned with web/src/api/parseTaskApi.ts maxTaskParseDepth.
const MaxTaskTreeDepth = tasks.MaxTreeDepth

// Get loads a task by id. See tasks.Get for the full contract.
func (s *Store) Get(ctx context.Context, id string) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Get")
	return tasks.Get(ctx, s.db, id)
}

// Create inserts a new task row, links any draft evaluations,
// removes the source draft, appends task_created (and parent
// subtask_added), and runs the checklist guard when the initial
// status is StatusDone — all in one transaction. Fires the
// ready-task notifier when the freshly created task is StatusReady.
func (s *Store) Create(ctx context.Context, in CreateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Create")
	t, err := tasks.Create(ctx, s.db, in, by)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	if t.Status != domain.StatusReady {
		return t, nil
	}
	if t.PickupNotBefore != nil && t.PickupNotBefore.After(now) {
		s.schedulePickupWake(ctx, t.ID, *t.PickupNotBefore)
		return t, nil
	}
	if ShouldNotifyReadyNow(t.PickupNotBefore, now) {
		s.notifyReadyTask(ctx, *t)
	}
	return t, nil
}

// Update applies the patch and notifies the ready-task channel when
// the task transitions into StatusReady. Also notifies when a Ready
// task's pickup_not_before patch makes it eligible right now (e.g.
// operator cleared the schedule or pulled it into the past) — the
// in-memory queue would otherwise stay empty until the next periodic
// reconcile tick. See tasks.Update for the per-field rules and
// docs/SCHEDULING.md for the two-queues invariant.
func (s *Store) Update(ctx context.Context, id string, in UpdateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Update")
	updated, prev, err := tasks.Update(ctx, s.db, id, in, by)
	if err != nil {
		return nil, err
	}
	if updated == nil || updated.Status != domain.StatusReady {
		if updated != nil {
			s.cancelPickupWake(updated.ID)
		}
		return updated, nil
	}
	now := time.Now().UTC()
	if updated.PickupNotBefore != nil && updated.PickupNotBefore.After(now) {
		s.schedulePickupWake(ctx, updated.ID, *updated.PickupNotBefore)
		return updated, nil
	}
	s.cancelPickupWake(updated.ID)
	transitionedToReady := prev != domain.StatusReady
	pickupTouched := in.PickupNotBefore != nil
	if transitionedToReady || pickupTouched {
		s.notifyReadyTask(ctx, *updated)
	}
	return updated, nil
}

// Delete removes the task at id and every descendant in one
// transaction. Returns the full set of removed task ids (root first,
// then BFS descendants) and the surviving grandparent id (or "" when
// the requested root had no parent) so the caller can fan out one
// task_deleted SSE event per id and one task_updated event for the
// surviving parent. See tasks.Delete for the full contract.
func (s *Store) Delete(ctx context.Context, id string, by domain.Actor) ([]string, string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Delete")
	deletedIDs, parent, err := tasks.Delete(ctx, s.db, id, by)
	if err != nil {
		return nil, "", err
	}
	for _, tid := range deletedIDs {
		s.cancelPickupWake(tid)
	}
	return deletedIDs, parent, nil
}

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
