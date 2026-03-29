package handler

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

// persistTaskUpdatedSSE applies a no-op priority update via the same store.Update path as PATCH /tasks
// (transaction, row lock, Save), then publishes task_updated on the hub — matching real mutations.
// The priority value is unchanged, so applyTaskPatches typically adds no audit row, but the update transaction still runs.
func persistTaskUpdatedSSE(ctx context.Context, st *store.Store, hub *SSEHub, id string) error {
	if st == nil || hub == nil {
		return errors.New("store or hub nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("task id: %w", domain.ErrInvalidInput)
	}
	t, err := st.Get(ctx, id)
	if err != nil {
		return err
	}
	pr := t.Priority
	_, err = st.Update(ctx, id, store.UpdateTaskInput{Priority: &pr}, domain.ActorUser)
	if err != nil {
		return err
	}
	hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: t.ID})
	return nil
}

// pickFirstTaskID returns the first task id in list order (id ASC), or false if none.
func pickFirstTaskID(ctx context.Context, st *store.Store) (id string, ok bool) {
	rows, err := st.List(ctx, 1, 0)
	if err != nil || len(rows) == 0 {
		return "", false
	}
	return rows[0].ID, true
}
