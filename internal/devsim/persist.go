package devsim

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

const listPage = 200 // store.ListFlat maximum page size

const logCmd = "taskapi"

// EventCycle is the full set of domain.EventType values used by the dev ticker, in display order.
// Keep in sync with pkgs/tasks/domain/enums.go (every EventType exactly once).
// Index 0 is chosen when len(events)%len(cycle)==0; index 1 is the first tick after task_created.
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
	domain.EventChecklistItemAdded,
	domain.EventChecklistItemToggled,
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
	case domain.EventContextAdded:
		return json.Marshal(map[string]string{"summary": "Repo layout", "detail": "Tasks live under pkgs/tasks."})
	case domain.EventConstraintAdded:
		return json.Marshal(map[string]string{"text": "Must keep default go test ./... green."})
	case domain.EventSuccessCriterionAdded:
		return json.Marshal(map[string]string{"text": "UI timeline renders without console errors."})
	case domain.EventNonGoalAdded:
		return json.Marshal(map[string]string{"text": "No production deploy in this iteration."})
	case domain.EventPlanAdded:
		return json.Marshal(map[string]any{
			"title": "Dev sim plan",
			"steps": []string{"Sketch", "Implement", "Verify"},
		})
	case domain.EventSubtaskAdded:
		return json.Marshal(map[string]string{
			"child_task_id": "00000000-0000-0000-0000-000000000099",
			"title":         "Child (synthetic id)",
		})
	case domain.EventChecklistItemAdded:
		return json.Marshal(map[string]string{"item_id": "cli-dev-1", "text": "Run go test ./..."})
	case domain.EventChecklistItemToggled:
		return json.Marshal(map[string]string{"item_id": "cli-dev-1", "done": "true"})
	case domain.EventArtifactAdded:
		return json.Marshal(map[string]string{"name": "notes.md", "uri": "file:///tmp/t2a-devsim"})
	case domain.EventApprovalRequested:
		return json.Marshal(map[string]string{"reason": "Checkpoint ready", "checkpoint": "plan_review"})
	case domain.EventApprovalGranted:
		return json.Marshal(map[string]string{"grantor": "lead", "note": "LGTM (synthetic)"})
	case domain.EventTaskCompleted:
		return json.Marshal(map[string]string{"summary": "Synthetic completion."})
	case domain.EventTaskFailed:
		return json.Marshal(map[string]string{"error": "Simulated failure", "retryable": "true"})
	case domain.EventSyncPing:
		return json.Marshal(map[string]string{"source": "devsim"})
	default:
		return json.Marshal(map[string]string{"dev_sample": string(typ)})
	}
}

func nextEventType(evs []domain.TaskEvent) domain.EventType {
	if len(EventCycle) == 0 {
		return domain.EventSyncPing
	}
	idx := len(evs) % len(EventCycle)
	return EventCycle[idx]
}

func persistSampleEvent(ctx context.Context, st *store.Store, t *domain.Task, opts Options, publish func(ChangeKind, string)) error {
	if st == nil || t == nil {
		return errors.New("store or task nil")
	}
	if publish == nil {
		return errors.New("publish nil")
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
	if opts.SyncTaskRow {
		if err := st.ApplyDevTaskRowMirror(ctx, t.ID, typ, payload); err != nil {
			slog.Debug("sse dev mirror skipped", "cmd", logCmd, "operation", "devsim.mirror_task",
				"task_id", t.ID, "type", typ, "err", err)
		}
	}
	if opts.UserResponse && domain.EventTypeAcceptsUserResponse(typ) {
		evs2, err := st.ListTaskEvents(ctx, t.ID)
		if err != nil {
			return err
		}
		if len(evs2) == 0 {
			return errors.New("no events after append")
		}
		seq := evs2[len(evs2)-1].Seq
		msg := "Synthetic user reply (devsim)."
		if typ == domain.EventTaskFailed {
			msg = "Synthetic triage note (devsim)."
		}
		if err := st.AppendTaskEventResponseMessage(ctx, t.ID, seq, msg, domain.ActorUser); err != nil {
			slog.Debug("sse dev user_response skipped", "cmd", logCmd, "operation", "devsim.user_response",
				"task_id", t.ID, "seq", seq, "err", err)
		}
	}
	publish(ChangeUpdated, t.ID)
	return nil
}

// PersistAllTasks walks every task using store.ListFlat (id ASC, paginated), same data as flat list.
// For each task it appends up to opts.EventsPerTick sample audit events and invokes publish after each successful cycle.
func PersistAllTasks(ctx context.Context, st *store.Store, opts Options, publish func(ChangeKind, string)) {
	if st == nil || publish == nil {
		return
	}
	per := opts.EventsPerTick
	if per < 1 {
		per = 1
	}
	if per > maxEventsPerTick {
		per = maxEventsPerTick
	}
	for offset := 0; ; offset += listPage {
		rows, err := st.ListFlat(ctx, listPage, offset)
		if err != nil {
			slog.Debug("sse dev ticker list failed", "cmd", logCmd, "operation", "devsim.tick_list", "err", err)
			return
		}
		for i := range rows {
			for range per {
				if err := persistSampleEvent(ctx, st, &rows[i], opts, publish); err != nil {
					slog.Debug("sse dev ticker task skipped", "cmd", logCmd, "operation", "devsim.tick_task",
						"task_id", rows[i].ID, "err", err)
					break
				}
			}
		}
		if len(rows) < listPage {
			return
		}
	}
}
