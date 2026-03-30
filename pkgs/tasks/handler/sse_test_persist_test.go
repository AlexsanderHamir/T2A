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

	// Same code path as PATCH: cyclic ready → running, status_changed by agent.
	for _, id := range []string{a.ID, b.ID} {
		tsk, err := st.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if tsk.Status != domain.StatusRunning {
			t.Fatalf("task %s status = %q want running", id, tsk.Status)
		}
		evs, err := st.ListTaskEvents(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if len(evs) < 2 {
			t.Fatalf("task %s: want >= 2 events, got %d", id, len(evs))
		}
		last := evs[len(evs)-1]
		if last.Type != domain.EventStatusChanged {
			t.Fatalf("task %s: last event type = %q want status_changed", id, last.Type)
		}
		if last.By != domain.ActorAgent {
			t.Fatalf("task %s: last event by = %q want agent", id, last.By)
		}
	}
}

func TestNextStatusForDevTicker(t *testing.T) {
	tests := []struct {
		cur  domain.Status
		want domain.Status
	}{
		{domain.StatusReady, domain.StatusRunning},
		{domain.StatusRunning, domain.StatusBlocked},
		{domain.StatusBlocked, domain.StatusReview},
		{domain.StatusReview, domain.StatusDone},
		{domain.StatusDone, domain.StatusFailed},
		{domain.StatusFailed, domain.StatusReady},
	}
	for _, tt := range tests {
		if got := nextStatusForDevTicker(tt.cur); got != tt.want {
			t.Fatalf("next(%q) = %q want %q", tt.cur, got, tt.want)
		}
	}
	if got := nextStatusForDevTicker("weird"); got != domain.StatusReady {
		t.Fatalf("next(unknown) = %q want ready", got)
	}
}
