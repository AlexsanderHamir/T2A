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
		domain.EventChecklistItemAdded:    {},
		domain.EventChecklistItemToggled:  {},
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

	var lines []devsim.ChangeKind
	publish := func(k devsim.ChangeKind, id string) {
		lines = append(lines, k)
		_ = id
	}

	devsim.PersistAllTasks(ctx, st, devsim.Options{}, publish)

	if len(lines) != 2 {
		t.Fatalf("want 2 publish calls, got %d (%v)", len(lines), lines)
	}
	for _, k := range lines {
		if k != devsim.ChangeUpdated {
			t.Fatalf("want ChangeUpdated, got %v", k)
		}
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

func TestDevSim_PersistAllTasks_burst_emitsMultiplePublishes(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	if _, err := st.Create(ctx, store.CreateTaskInput{Title: "solo"}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	var n int
	devsim.PersistAllTasks(ctx, st, devsim.Options{EventsPerTick: 3}, func(devsim.ChangeKind, string) { n++ })
	if n != 3 {
		t.Fatalf("want 3 publishes, got %d", n)
	}
}

func TestDevSim_PersistAllTasks_syncRow_updatesStatus(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	tsk, err := st.Create(ctx, store.CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	devsim.PersistAllTasks(ctx, st, devsim.Options{SyncTaskRow: true}, func(devsim.ChangeKind, string) {})
	got, err := st.Get(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusRunning {
		t.Fatalf("status %q want running after mirror", got.Status)
	}
}

func TestDevSim_userResponse_appendsThread(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	tsk, err := st.Create(ctx, store.CreateTaskInput{Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	reached := false
	for range 200 {
		n, err := st.TaskEventCount(ctx, tsk.ID)
		if err != nil {
			t.Fatal(err)
		}
		if devsimNextTypeFromCount(n) == domain.EventApprovalRequested {
			reached = true
			break
		}
		devsim.PersistAllTasks(ctx, st, devsim.Options{}, func(devsim.ChangeKind, string) {})
	}
	if !reached {
		t.Fatal("did not reach approval_requested in event cycle")
	}
	devsim.PersistAllTasks(ctx, st, devsim.Options{UserResponse: true}, func(devsim.ChangeKind, string) {})
	evs, err := st.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	last := evs[len(evs)-1]
	if last.Type != domain.EventApprovalRequested {
		t.Fatalf("last type %q want approval_requested", last.Type)
	}
	entries := store.ThreadEntriesForDisplay(&last)
	if len(entries) == 0 {
		t.Fatal("expected response thread entries")
	}
	if entries[len(entries)-1].By != domain.ActorUser {
		t.Fatalf("want user message, got %+v", entries[len(entries)-1])
	}
}

// devsimNextTypeFromCount mirrors internal/devsim.nextEventTypeFromCount for tests.
func devsimNextTypeFromCount(n int64) domain.EventType {
	if len(devsim.EventCycle) == 0 {
		return domain.EventSyncPing
	}
	idx := int(n % int64(len(devsim.EventCycle)))
	return devsim.EventCycle[idx]
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
	devsim.PersistAllTasks(ctx, st, devsim.Options{}, func(devsim.ChangeKind, string) { saw = true })
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
