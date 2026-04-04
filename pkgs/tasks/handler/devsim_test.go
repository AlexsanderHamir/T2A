package handler

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/devsim"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestDevSim_EventCycle_exhaustive(t *testing.T) {
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
	if len(devsim.EventCycle) != len(want) {
		t.Fatalf("EventCycle len %d want %d", len(devsim.EventCycle), len(want))
	}
	seen := make(map[domain.EventType]int)
	for _, e := range devsim.EventCycle {
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

func TestDevSim_PersistAllTasks_emitsOnePublishPerTask(t *testing.T) {
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

	var lines []string
	publish := func(id string) { lines = append(lines, id) }

	devsim.PersistAllTasks(ctx, st, publish)

	if len(lines) != 2 {
		t.Fatalf("want 2 publish calls, got %d (%v)", len(lines), lines)
	}
	counts := map[string]int{}
	for _, id := range lines {
		counts[id]++
	}
	if counts[a.ID] != 1 || counts[b.ID] != 1 {
		t.Fatalf("want one publish per task, got %+v", counts)
	}

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
		if last.Type != devsim.EventCycle[1] {
			t.Fatalf("task %s: last event type = %q want %q", id, last.Type, devsim.EventCycle[1])
		}
		if last.By != domain.ActorAgent {
			t.Fatalf("task %s: last event by = %q want agent", id, last.By)
		}
	}
}

func TestDevSim_SamplePayload_JSON(t *testing.T) {
	// Exercise sample JSON shape via one persisted event (status_changed payload).
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	task, err := st.Create(ctx, store.CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	devsim.PersistAllTasks(ctx, st, func(string) { saw = true })
	if !saw {
		t.Fatal("expected publish")
	}
	evs, err := st.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("want 2 events, got %d", len(evs))
	}
	last := evs[len(evs)-1]
	if last.Type != domain.EventStatusChanged {
		t.Fatalf("last type %q want %q", last.Type, domain.EventStatusChanged)
	}
	var m map[string]string
	if err := json.Unmarshal(last.Data, &m); err != nil {
		t.Fatal(err)
	}
	if m["from"] != "ready" || m["to"] != "running" {
		t.Fatalf("got %+v", m)
	}
}
