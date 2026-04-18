package store

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store/internal/tasks"
)

// CreateTaskInput is the public re-export of the task creation
// payload. The alias keeps every existing call-site unchanged while
// the implementation lives in internal/tasks.
type CreateTaskInput = tasks.CreateInput

// UpdateTaskInput is the public re-export of the task patch payload.
type UpdateTaskInput = tasks.UpdateInput

// ParentFieldPatch is the public re-export of the parent-id patch
// helper used by UpdateTaskInput.Parent.
type ParentFieldPatch = tasks.ParentFieldPatch

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
	if t.Status == domain.StatusReady {
		s.notifyReadyTask(ctx, *t)
	}
	return t, nil
}

// Update applies the patch and notifies the ready-task channel when
// the task transitions into StatusReady. See tasks.Update for the
// per-field rules.
func (s *Store) Update(ctx context.Context, id string, in UpdateTaskInput, by domain.Actor) (*domain.Task, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Update")
	updated, prev, err := tasks.Update(ctx, s.db, id, in, by)
	if err != nil {
		return nil, err
	}
	if updated != nil && updated.Status == domain.StatusReady && prev != domain.StatusReady {
		s.notifyReadyTask(ctx, *updated)
	}
	return updated, nil
}

// Delete removes a leaf task and returns the parent id (or "" for
// roots) so the caller can fan out an SSE poke. See tasks.Delete for
// the full contract.
func (s *Store) Delete(ctx context.Context, id string, by domain.Actor) (string, error) {
	slog.Debug("trace", "cmd", storeLogCmd, "operation", "tasks.store.Delete")
	return tasks.Delete(ctx, s.db, id, by)
}
