package handler

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const sseTestListPage = 200 // store.List maximum page size

// sseTestEventCycle is the full set of domain.EventType values used by the dev ticker, in display order.
// Keep in sync with pkgs/tasks/domain/enums.go (every EventType exactly once).
// Index 0 is chosen when len(events)%len==0 (e.g. 17th append); index 1 is the first tick after task_created.
var sseTestEventCycle = []domain.EventType{
	domain.EventTaskCreated,
	domain.EventStatusChanged,
	domain.EventPriorityChanged,
	domain.EventPromptAppended,
	domain.EventMessageAdded,
	domain.EventContextAdded,
	domain.EventConstraintAdded,
	domain.EventSuccessCriterionAdded,
	domain.EventNonGoalAdded,
	domain.EventPlanAdded,
	domain.EventSubtaskAdded,
	domain.EventArtifactAdded,
	domain.EventApprovalRequested,
	domain.EventApprovalGranted,
	domain.EventTaskCompleted,
	domain.EventTaskFailed,
	domain.EventSyncPing,
}

func sseTestEventPayload(typ domain.EventType) ([]byte, error) {
	switch typ {
	case domain.EventStatusChanged:
		return json.Marshal(map[string]string{"from": "ready", "to": "running"})
	case domain.EventPriorityChanged:
		return json.Marshal(map[string]string{"from": "medium", "to": "high"})
	case domain.EventPromptAppended:
		return json.Marshal(map[string]string{"from": "<p>a</p>", "to": "<p>a</p><p>b</p>"})
	case domain.EventMessageAdded:
		return json.Marshal(map[string]string{"from": "Title A", "to": "Title B"})
	default:
		return json.Marshal(map[string]string{"dev_sample": string(typ)})
	}
}

// devTickerNextEventType picks the next event type from sseTestEventCycle using the current event count
// so each append advances through all allowed types without package-global state (tests stay deterministic).
func devTickerNextEventType(evs []domain.TaskEvent) domain.EventType {
	if len(sseTestEventCycle) == 0 {
		return domain.EventSyncPing
	}
	idx := len(evs) % len(sseTestEventCycle)
	return sseTestEventCycle[idx]
}

// persistDevTickerSampleEvent appends the next rotated audit event (ActorAgent) via store.AppendTaskEvent,
// then publishes task_updated. Does not mutate the task row—only the audit log.
func persistDevTickerSampleEvent(ctx context.Context, st *store.Store, hub *SSEHub, t *domain.Task) error {
	if st == nil || hub == nil || t == nil {
		return errors.New("store, hub, or task nil")
	}
	evs, err := st.ListTaskEvents(ctx, t.ID)
	if err != nil {
		return err
	}
	typ := devTickerNextEventType(evs)
	payload, err := sseTestEventPayload(typ)
	if err != nil {
		return err
	}
	if err := st.AppendTaskEvent(ctx, t.ID, typ, domain.ActorAgent, payload); err != nil {
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
			if err := persistDevTickerSampleEvent(ctx, st, hub, &rows[i]); err != nil {
				slog.Debug("sse dev ticker task skipped", "cmd", httpLogCmd, "operation", "tasks.sse_test.tick_task",
					"task_id", rows[i].ID, "err", err)
			}
		}
		if len(rows) < sseTestListPage {
			return
		}
	}
}
