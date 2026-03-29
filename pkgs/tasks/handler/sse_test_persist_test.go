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
}
