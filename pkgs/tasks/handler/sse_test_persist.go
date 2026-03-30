package handler

import (
	"context"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const sseTestListPage = 200 // store.List maximum page size

// nextStatusForDevTicker returns the next status in a fixed cycle so each tick produces a real
// status_changed audit row (same as PATCH) instead of a synthetic sync_ping.
func nextStatusForDevTicker(cur domain.Status) domain.Status {
	switch cur {
	case domain.StatusReady:
		return domain.StatusRunning
	case domain.StatusRunning:
		return domain.StatusBlocked
	case domain.StatusBlocked:
		return domain.StatusReview
	case domain.StatusReview:
		return domain.StatusDone
	case domain.StatusDone:
		return domain.StatusFailed
	case domain.StatusFailed:
		return domain.StatusReady
	default:
		return domain.StatusReady
	}
}

// persistDevTickerTaskUpdate applies one dev-only status transition via store.Update (ActorAgent),
// emitting the same status_changed event + task row change as a normal API patch, then publishes
// task_updated on the hub.
func persistDevTickerTaskUpdate(ctx context.Context, st *store.Store, hub *SSEHub, t *domain.Task) error {
	if st == nil || hub == nil || t == nil {
		return errors.New("store, hub, or task nil")
	}
	next := nextStatusForDevTicker(t.Status)
	_, err := st.Update(ctx, t.ID, store.UpdateTaskInput{Status: &next}, domain.ActorAgent)
	if err != nil {
		return err
	}
	hub.Publish(TaskChangeEvent{Type: TaskUpdated, ID: t.ID})
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
			if err := persistDevTickerTaskUpdate(ctx, st, hub, &rows[i]); err != nil {
				slog.Debug("sse dev ticker task skipped", "cmd", httpLogCmd, "operation", "tasks.sse_test.tick_task",
					"task_id", rows[i].ID, "err", err)
			}
		}
		if len(rows) < sseTestListPage {
			return
		}
	}
}
