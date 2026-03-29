package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const sseTestListPage = 200 // store.List maximum page size

// persistTaskUpdatedSSE appends a sync_ping audit row (same persistence as real timeline entries), then
// publishes task_updated on the hub so clients refetch list and GET /tasks/{id}/events.
func persistTaskUpdatedSSE(ctx context.Context, st *store.Store, hub *SSEHub, id string) error {
	if st == nil || hub == nil {
		return errors.New("store or hub nil")
	}
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("task id: %w", domain.ErrInvalidInput)
	}
	if err := st.AppendTaskEvent(ctx, id, domain.EventSyncPing, domain.ActorUser, nil); err != nil {
		return err
	}
	hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: id})
	return nil
}

// persistAllTasksForSSETest walks every task using store.List (id ASC, paginated), same data as GET /tasks.
func persistAllTasksForSSETest(ctx context.Context, st *store.Store, hub *SSEHub) {
	if st == nil || hub == nil {
		return
	}
	for offset := 0; ; offset += sseTestListPage {
		rows, err := st.List(ctx, sseTestListPage, offset)
		if err != nil {
			slog.Debug("sse dev ticker list failed", "cmd", httpLogCmd, "operation", "tasks.sse_test.tick_list", "err", err)
			return
		}
		for i := range rows {
			if err := persistTaskUpdatedSSE(ctx, st, hub, rows[i].ID); err != nil {
				slog.Debug("sse dev ticker task skipped", "cmd", httpLogCmd, "operation", "tasks.sse_test.tick_task",
					"task_id", rows[i].ID, "err", err)
			}
		}
		if len(rows) < sseTestListPage {
			return
		}
	}
}
