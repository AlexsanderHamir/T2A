package devsim

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const listPage = 200 // store.List maximum page size

const logCmd = "taskapi"

// EventCycle is the full set of domain.EventType values used by the dev ticker, in display order.
// Keep in sync with pkgs/tasks/domain/enums.go (every EventType exactly once).
// Index 0 is chosen when len(events)%len==0 (e.g. 17th append); index 1 is the first tick after task_created.
var EventCycle = []domain.EventType{
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

func samplePayload(typ domain.EventType) ([]byte, error) {
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

// nextEventType picks the next event type from EventCycle using the current event count
// so each append advances through all allowed types without package-global state (tests stay deterministic).
func nextEventType(evs []domain.TaskEvent) domain.EventType {
	if len(EventCycle) == 0 {
		return domain.EventSyncPing
	}
	idx := len(evs) % len(EventCycle)
	return EventCycle[idx]
}

// persistSampleEvent appends the next rotated audit event (ActorAgent) via store.AppendTaskEvent,
// then calls publish with the task id. Does not mutate the task row—only the audit log.
func persistSampleEvent(ctx context.Context, st *store.Store, t *domain.Task, publish func(taskID string)) error {
	if st == nil || t == nil {
		return errors.New("store or task nil")
	}
	evs, err := st.ListTaskEvents(ctx, t.ID)
	if err != nil {
		return err
	}
	typ := nextEventType(evs)
	payload, err := samplePayload(typ)
	if err != nil {
		return err
	}
	if err := st.AppendTaskEvent(ctx, t.ID, typ, domain.ActorAgent, payload); err != nil {
		return err
	}
	if publish != nil {
		publish(t.ID)
	}
	return nil
}

// PersistAllTasks walks every task using store.List (id ASC, paginated), same data as GET /tasks.
// For each task it appends one sample audit event and invokes publish after each successful append.
func PersistAllTasks(ctx context.Context, st *store.Store, publish func(taskID string)) {
	if st == nil {
		return
	}
	for offset := 0; ; offset += listPage {
		rows, err := st.List(ctx, listPage, offset)
		if err != nil {
			slog.Debug("sse dev ticker list failed", "cmd", logCmd, "operation", "devsim.tick_list", "err", err)
			return
		}
		for i := range rows {
			if err := persistSampleEvent(ctx, st, &rows[i], publish); err != nil {
				slog.Debug("sse dev ticker task skipped", "cmd", logCmd, "operation", "devsim.tick_task",
					"task_id", rows[i].ID, "err", err)
			}
		}
		if len(rows) < listPage {
			return
		}
	}
}
