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
	domain.EventSubtaskRemoved,
	domain.EventChecklistItemAdded,
	domain.EventChecklistItemToggled,
	domain.EventChecklistItemUpdated,
	domain.EventChecklistItemRemoved,
	domain.EventChecklistInheritChanged,
	domain.EventArtifactAdded,
	domain.EventApprovalRequested,
	domain.EventApprovalGranted,
	domain.EventTaskCompleted,
	domain.EventTaskFailed,
	domain.EventSyncPing,
}

func samplePayloadForType(typ domain.EventType) ([]byte, error) {
	slog.Debug("trace", "cmd", logCmd, "operation", "devsim.samplePayloadForType", "type", typ)
	if f, ok := samplePayloadByType[typ]; ok {
		return f()
	}
	return json.Marshal(map[string]string{"dev_sample": string(typ)})
}

func nextEventTypeFromCount(n int64) domain.EventType {
	slog.Debug("trace", "cmd", logCmd, "operation", "devsim.nextEventTypeFromCount")
	if len(EventCycle) == 0 {
		return domain.EventSyncPing
	}
	idx := int(n % int64(len(EventCycle)))
	return EventCycle[idx]
}

func persistSampleEvent(ctx context.Context, st *store.Store, t *domain.Task, opts Options, publish func(ChangeKind, string)) error {
	if st == nil || t == nil {
		return errors.New("store or task nil")
	}
	if publish == nil {
		return errors.New("publish nil")
	}
	n, err := st.TaskEventCount(ctx, t.ID)
	if err != nil {
		return err
	}
	typ := nextEventTypeFromCount(n)
	payload, err := samplePayloadForType(typ)
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
		seq, err := st.LastEventSeq(ctx, t.ID)
		if err != nil {
			return err
		}
		if seq < 1 {
			return errors.New("no events after append")
		}
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
