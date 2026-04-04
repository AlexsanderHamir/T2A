package devsim

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestEventCycle_exhaustive(t *testing.T) {
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
	if len(EventCycle) != len(want) {
		t.Fatalf("EventCycle len %d want %d", len(EventCycle), len(want))
	}
	seen := make(map[domain.EventType]int)
	for _, e := range EventCycle {
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

func TestPersistAllTasks_emitsOnePublishPerTask(t *testing.T) {
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

	var lines []ChangeKind
	publish := func(k ChangeKind, id string) {
		lines = append(lines, k)
		_ = id
	}

	PersistAllTasks(ctx, st, Options{}, publish)

	if len(lines) != 2 {
		t.Fatalf("want 2 publish calls, got %d (%v)", len(lines), lines)
	}
	for _, k := range lines {
		if k != ChangeUpdated {
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
		if last.Type != EventCycle[1] {
			t.Fatalf("task %s: last event type = %q want %q", id, last.Type, EventCycle[1])
		}
		if last.By != domain.ActorAgent {
			t.Fatalf("task %s: last event by = %q want agent", id, last.By)
		}
	}
}

func TestPersistAllTasks_burst_emitsMultiplePublishes(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	if _, err := st.Create(ctx, store.CreateTaskInput{Title: "solo"}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	var n int
	PersistAllTasks(ctx, st, Options{EventsPerTick: 3}, func(ChangeKind, string) { n++ })
	if n != 3 {
		t.Fatalf("want 3 publishes, got %d", n)
	}
}

func TestPersistAllTasks_syncRow_updatesStatus(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	tsk, err := st.Create(ctx, store.CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	PersistAllTasks(ctx, st, Options{SyncTaskRow: true}, func(ChangeKind, string) {})
	got, err := st.Get(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusRunning {
		t.Fatalf("status %q want running after mirror", got.Status)
	}
}

func TestPersistAllTasks_userResponse_appendsThread(t *testing.T) {
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
		if nextEventTypeFromCount(n) == domain.EventApprovalRequested {
			reached = true
			break
		}
		PersistAllTasks(ctx, st, Options{}, func(ChangeKind, string) {})
	}
	if !reached {
		t.Fatal("did not reach approval_requested in event cycle")
	}
	PersistAllTasks(ctx, st, Options{UserResponse: true}, func(ChangeKind, string) {})
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

func TestSamplePayload_JSON(t *testing.T) {
	db := testdb.OpenSQLite(t)
	st := store.NewStore(db)
	ctx := context.Background()
	task, err := st.Create(ctx, store.CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	PersistAllTasks(ctx, st, Options{}, func(ChangeKind, string) { saw = true })
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
