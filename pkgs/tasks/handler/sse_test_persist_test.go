package handler

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestSseTestEventCycle_exhaustive(t *testing.T) {
	want := map[domain.EventType]struct{}{
		domain.EventTaskCreated:           {},
		domain.EventStatusChanged:         {},
		domain.EventPriorityChanged:       {},
		domain.EventPromptAppended:        {},
		domain.EventContextAdded:          {},
		domain.EventConstraintAdded:       {},
		domain.EventSuccessCriterionAdded: {},
		domain.EventNonGoalAdded:          {},
		domain.EventPlanAdded:             {},
		domain.EventSubtaskAdded:          {},
		domain.EventMessageAdded:          {},
		domain.EventArtifactAdded:         {},
		domain.EventApprovalRequested:     {},
		domain.EventApprovalGranted:       {},
		domain.EventTaskCompleted:         {},
		domain.EventTaskFailed:            {},
		domain.EventSyncPing:              {},
	}
	if len(sseTestEventCycle) != len(want) {
		t.Fatalf("sseTestEventCycle len %d want %d", len(sseTestEventCycle), len(want))
	}
	seen := make(map[domain.EventType]int)
	for _, e := range sseTestEventCycle {
		seen[e]++
		delete(want, e)
	}
	if len(want) != 0 {
		t.Fatalf("missing event types in cycle: %v", want)
	}
	for e, n := range seen {
		if n != 1 {
			t.Fatalf("duplicate %q count %d", e, n)
		}
	}
}

func TestPersistAllTasksForSSETest_emitsOneSSEPerTask(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	a, err := st.Create(ctx, store.CreateTaskInput{Title: "a"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := st.Create(ctx, store.CreateTaskInput{Title: "b"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	hub := NewSSEHub()
	ch, cancel := hub.Subscribe()
	defer cancel()

	persistAllTasksForSSETest(ctx, st, hub)

	want := map[string]bool{a.ID: false, b.ID: false}
	for range want {
		select {
		case line := <-ch:
			var ev TaskChangeEvent
			if err := json.Unmarshal([]byte(line), &ev); err != nil {
				t.Fatal(err)
			}
			if ev.Type != TaskUpdated {
				t.Fatalf("want task_updated, got %q", ev.Type)
			}
			if _, ok := want[ev.ID]; !ok {
				t.Fatalf("unexpected id %q", ev.ID)
			}
			want[ev.ID] = true
		case <-time.After(3 * time.Second):
			t.Fatal("timeout waiting for hub events")
		}
	}
	for id, seen := range want {
		if !seen {
			t.Fatalf("missing event for %s", id)
		}
	}

	// One task_created from Create; first dev append uses len(events)==1 → cycle[1].

	for _, id := range []string{a.ID, b.ID} {
		tsk, err := st.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if tsk.Status != domain.StatusReady {
			t.Fatalf("task %s status = %q want ready (ticker does not patch task row)", id, tsk.Status)
		}
		evs, err := st.ListTaskEvents(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) != 2 {
			t.Fatalf("task %s: want 2 events, got %d", id, len(evs))
		}
		last := evs[len(evs)-1]
		if last.Type != sseTestEventCycle[1] {
			t.Fatalf("task %s: last event type = %q want %q", id, last.Type, sseTestEventCycle[1])
		}
		if last.By != domain.ActorAgent {
			t.Fatalf("task %s: last event by = %q want agent", id, last.By)
		}
	}
}

func TestDevTickerNextEventType(t *testing.T) {
	if n := len(sseTestEventCycle); n < 3 {
		t.Fatalf("cycle too short: %d", n)
	}
	evs := make([]domain.TaskEvent, 2)
	if got := devTickerNextEventType(evs); got != sseTestEventCycle[2%len(sseTestEventCycle)] {
		t.Fatalf("len=2: got %q want %q", got, sseTestEventCycle[2])
	}
}
