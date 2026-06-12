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
// immediately by the worker — see docs/data-model.md ("the two queues").
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

// ProjectFieldPatch is the public re-export of the project-id patch
// helper used by UpdateTaskInput.Project.
type ProjectFieldPatch = tasks.ProjectFieldPatch

// PickupNotBeforePatch is the public re-export of the
// pickup_not_before patch helper used by UpdateTaskInput.PickupNotBefore.
// See docs/data-model.md.
type PickupNotBeforePatch = tasks.PickupNotBeforePatch

// TaskNode is a task row plus nested children for API tree responses. It
// re-exports internal/tasks.Node.
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
// docs/data-model.md for the two-queues invariant.
func (s *Store) Update(ctx context.Context, id string, in UpdateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Update")
	updated, prev, err := tasks.Update(ctx, s.db, id, in, by)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, nil
	}

	if updated.Status == domain.StatusDone && prev != domain.StatusDone {
		s.notifyUnblockedDependents(ctx, updated.ID)
		if parentID, autoErr := tasks.TryAutoCompleteParent(ctx, s.db, updated.ID, by); autoErr != nil {
			slog.Warn("auto-complete parent after subtask done", "task_id", updated.ID, "err", autoErr)
		} else if parentID != "" {
			s.notifyUnblockedDependents(ctx, parentID)
		}
	}

	if updated.Status != domain.StatusReady {
		s.cancelPickupWake(updated.ID)
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

func (s *Store) notifyUnblockedDependents(ctx context.Context, predecessorID string) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.notifyUnblockedDependents", "predecessor_id", predecessorID)
	dependents, err := tasks.ListDependents(ctx, s.db, predecessorID)
	if err != nil {
		slog.Warn("list dependents after predecessor unblock", "task_id", predecessorID, "err", err)
		return
	}
	now := time.Now().UTC()
	for _, id := range dependents {
		t, err := tasks.Get(ctx, s.db, id)
		if err != nil {
			continue
		}
		ok, err := tasks.ReadyForAgentPickup(ctx, s.db, t, now)
		if err != nil || !ok {
			continue
		}
		s.notifyReadyTask(ctx, *t)
	}
}

// NotifyUnblockedDependents wakes dependents whose dependency edges are
// now satisfied. Used after checklist criteria become complete.
func (s *Store) NotifyUnblockedDependents(ctx context.Context, predecessorID string) {
	s.notifyUnblockedDependents(ctx, predecessorID)
}

// HasIncompleteSubtasks reports whether taskID has direct children not done.
func (s *Store) HasIncompleteSubtasks(ctx context.Context, taskID string) (bool, error) {
	return tasks.HasIncompleteSubtasks(ctx, s.db, taskID)
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

// ListFilter is the public re-export for optional flat-list filters.
type ListFilter = tasks.ListFilter

// ListFlat returns tasks ordered by id ASC with limit/offset over
// all rows (no tree). See tasks.ListFlat for clamp rules.
func (s *Store) ListFlat(ctx context.Context, limit, offset int, filter *ListFilter) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListFlat")
	return tasks.ListFlat(ctx, s.db, limit, offset, filter)
}

// List is an alias for ListFlat. Prefer ListFlat in new code.
func (s *Store) List(ctx context.Context, limit, offset int) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.List")
	return tasks.ListFlat(ctx, s.db, limit, offset, nil)
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

func (s *Store) AddTaskDependency(ctx context.Context, taskID, dependsOnTaskID string, satisfies domain.DependencySatisfies) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AddTaskDependency")
	return tasks.AddDependency(ctx, s.db, taskID, dependsOnTaskID, satisfies)
}

func (s *Store) RemoveTaskDependency(ctx context.Context, taskID, dependsOnTaskID string) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.RemoveTaskDependency")
	return tasks.RemoveDependency(ctx, s.db, taskID, dependsOnTaskID)
}

func (s *Store) ListTaskDependencies(ctx context.Context, taskID string) ([]domain.DependencyEdge, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListTaskDependencies")
	return tasks.ListDependencyEdges(ctx, s.db, taskID)
}

func (s *Store) SetTaskDependencies(ctx context.Context, taskID string, dependsOn []domain.DependencyEdge) error {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.SetTaskDependencies")
	return tasks.SetDependencies(ctx, s.db, taskID, dependsOn)
}

// ReadyForAgentPickup reports whether the task passes dequeue predicates.
func (s *Store) ReadyForAgentPickup(ctx context.Context, t *domain.Task, now time.Time) (bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ReadyForAgentPickup")
	return tasks.ReadyForAgentPickup(ctx, s.db, t, now)
}

// ApplyTaskGateAction applies release/hold/clear_hold to a task gate.
func (s *Store) ApplyTaskGateAction(ctx context.Context, taskID, action string, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApplyTaskGateAction")
	return tasks.ApplyTaskGateAction(ctx, s.db, taskID, action, by)
}
