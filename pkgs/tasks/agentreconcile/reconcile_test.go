package agentreconcile

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/agents"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestReconcileReadyTasksNotQueued_enqueuesMissing(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)

	t1, err := st.Create(ctx, store.CreateTaskInput{Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	t2, err := st.Create(ctx, store.CreateTaskInput{Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := q.NotifyReadyTask(ctx, *t1); err != nil {
		t.Fatal(err)
	}

	res, err := agents.ReconcileReadyTasksNotQueued(ctx, st, q, 50)
	if err != nil {
		t.Fatal(err)
	}
	if res.Enqueued != 1 {
		t.Fatalf("enqueued %d want 1", res.Enqueued)
	}
	if res.SkippedAlreadyQueued != 1 {
		t.Fatalf("skipped %d want 1", res.SkippedAlreadyQueued)
	}
	if res.Scanned < 2 {
		t.Fatalf("scanned %d", res.Scanned)
	}

	var got [2]domain.Task
	for i := range got {
		select {
		case got[i] = <-q.Recv():
			q.AckAfterRecv(got[i].ID)
		case <-time.After(2 * time.Second):
			t.Fatal("timeout recv")
		}
	}
	seen := map[string]bool{got[0].ID: true, got[1].ID: true}
	if !seen[t1.ID] || !seen[t2.ID] {
		t.Fatalf("got ids %q %q want %q %q", got[0].ID, got[1].ID, t1.ID, t2.ID)
	}
}

func TestReconcileReadyTasksNotQueued_nilStore(t *testing.T) {
	_, err := agents.ReconcileReadyTasksNotQueued(context.Background(), nil, agents.NewMemoryQueue(2), 10)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestReconcileReadyTasksNotQueued_stopsOnFull(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(1)
	if _, err := st.Create(ctx, store.CreateTaskInput{Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Create(ctx, store.CreateTaskInput{Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if err := q.NotifyReadyTask(ctx, domain.Task{ID: "00000000-0000-4000-8000-000000000001", Title: "stub", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}); err != nil {
		t.Fatal(err)
	}

	res, err := agents.ReconcileReadyTasksNotQueued(ctx, st, q, 50)
	if err != nil {
		t.Fatal(err)
	}
	if !res.StoppedOnQueueFull {
		t.Fatalf("want stopped on full, got %+v", res)
	}
}
