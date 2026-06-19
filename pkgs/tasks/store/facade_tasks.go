package store

import (
	"context"
	"log/slog"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/scheduling"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks"
)

// FailedPredicate identifies the first worker readiness check that failed.
type FailedPredicate = scheduling.FailedPredicate

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
	return scheduling.ShouldNotifyReadyNow(pickupNotBefore, now)
}

// CreateTaskInput is the public re-export of the task creation payload.
type CreateTaskInput = tasks.CreateInput

// UpdateTaskInput is the public re-export of the task patch payload.
type UpdateTaskInput = tasks.UpdateInput

// ProjectFieldPatch is the public re-export of the project-id patch helper.
type ProjectFieldPatch = tasks.ProjectFieldPatch

// PickupNotBeforePatch is the public re-export of the pickup_not_before patch helper.
type PickupNotBeforePatch = tasks.PickupNotBeforePatch

// RequestRetryInput is the public re-export of the operator retry payload.
type RequestRetryInput = tasks.RequestRetryInput

// AgentPickupResult is the public re-export of the worker pickup payload.
type AgentPickupResult = tasks.AgentPickupResult

// Get loads a task by id. See tasks.Get for the full contract.
func (s *Store) Get(ctx context.Context, id string) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Get")
	return tasks.Get(ctx, s.db, id)
}

// AgentPickup transitions ready→running and consumes pending_retry. Used by
// the agent worker at dequeue time instead of a bare status patch.
func (s *Store) AgentPickup(ctx context.Context, taskID string, by domain.Actor) (*AgentPickupResult, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.AgentPickup", "task_id", taskID)
	return tasks.AgentPickup(ctx, s.db, taskID, by)
}

// RequestTaskRetry queues operator retry intent for a failed task.
func (s *Store) RequestTaskRetry(ctx context.Context, in tasks.RequestRetryInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.RequestTaskRetry", "task_id", in.TaskID)
	updated, prev, err := tasks.RequestTaskRetry(ctx, s.db, in, by)
	if err != nil {
		return nil, err
	}
	if updated == nil {
		return nil, nil
	}
	now := time.Now().UTC()
	s.applyNotifyDecision(ctx, *updated, scheduling.DecideNotifyAfterReadyTransition(prev, updated, false, now))
	return updated, nil
}

// Create inserts a new task row, links draft evaluations, removes the
// source draft, appends task_created, and runs the checklist guard when
// the initial status is StatusDone — all in one transaction.
func (s *Store) Create(ctx context.Context, in CreateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Create")
	t, err := tasks.Create(ctx, s.db, in, by)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	s.applyNotifyDecision(ctx, *t, scheduling.DecideNotifyAfterReadyTransition("", t, false, now))
	return t, nil
}

// Update applies the patch and notifies the ready-task channel when appropriate.
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
	}

	now := time.Now().UTC()
	pickupTouched := in.PickupNotBefore != nil
	s.applyNotifyDecision(ctx, *updated, scheduling.DecideNotifyAfterReadyTransition(prev, updated, pickupTouched, now))
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
		ok, _, err := tasks.ReadyForAgentPickup(ctx, s.db, t, now)
		if err != nil || !ok {
			continue
		}
		s.notifyReadyTask(ctx, *t)
	}
}

// NotifyUnblockedDependents wakes dependents whose dependency edges are now satisfied.
func (s *Store) NotifyUnblockedDependents(ctx context.Context, predecessorID string) {
	s.notifyUnblockedDependents(ctx, predecessorID)
}

// Delete removes the task at id. Returns the deleted id on success.
func (s *Store) Delete(ctx context.Context, id string, by domain.Actor) ([]string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Delete")
	deletedIDs, err := tasks.Delete(ctx, s.db, id, by)
	if err != nil {
		return nil, err
	}
	for _, tid := range deletedIDs {
		s.cancelPickupWake(tid)
	}
	return deletedIDs, nil
}

// ListFilter is the public re-export for optional flat-list filters.
type ListFilter = tasks.ListFilter

// ListFlat returns tasks ordered by id ASC with limit/offset.
func (s *Store) ListFlat(ctx context.Context, limit, offset int, filter *ListFilter) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListFlat")
	return tasks.ListFlat(ctx, s.db, limit, offset, filter)
}

// ListFlatPage returns a flat page with hasMore.
func (s *Store) ListFlatPage(ctx context.Context, limit, offset int, filter *ListFilter) ([]domain.Task, bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListFlatPage")
	return tasks.ListFlatPage(ctx, s.db, limit, offset, filter)
}

// ListFlatAfter is the keyset-pagination variant of ListFlat.
func (s *Store) ListFlatAfter(ctx context.Context, limit int, afterID string) ([]domain.Task, bool, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ListFlatAfter")
	return tasks.ListFlatAfter(ctx, s.db, limit, afterID)
}

// List is an alias for ListFlat. Prefer ListFlat in new code.
func (s *Store) List(ctx context.Context, limit, offset int) ([]domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.List")
	return tasks.ListFlat(ctx, s.db, limit, offset, nil)
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
func (s *Store) ReadyForAgentPickup(ctx context.Context, t *domain.Task, now time.Time) (bool, FailedPredicate, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ReadyForAgentPickup")
	return tasks.ReadyForAgentPickup(ctx, s.db, t, now)
}

//funclogmeasure:skip category=hot-path reason="Pure helper without I/O; operation trace is emitted by the calling chokepoint."
func (s *Store) applyNotifyDecision(ctx context.Context, task domain.Task, d scheduling.NotifyDecision) {
	if d.ScheduleWake != nil {
		s.schedulePickupWake(ctx, task.ID, *d.ScheduleWake)
		return
	}
	if d.CancelWake {
		s.cancelPickupWake(task.ID)
	}
	if d.NotifyQueue {
		s.notifyReadyTask(ctx, task)
	}
}

// ApplyTaskGateAction applies release/hold/clear_hold to a task gate.
func (s *Store) ApplyTaskGateAction(ctx context.Context, taskID, action string, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.ApplyTaskGateAction")
	return tasks.ApplyTaskGateAction(ctx, s.db, taskID, action, by)
}
