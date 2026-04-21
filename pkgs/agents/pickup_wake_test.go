package agents

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestPickupWakeScheduler_WakeEnqueuesNearFutureTask(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := NewMemoryQueue(32)
	st.SetReadyTaskNotifier(q)
	w := NewPickupWakeScheduler(st, q)
	st.SetPickupWake(w)
	defer w.Stop()

	future := time.Now().UTC().Add(40 * time.Millisecond)
	tk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "wake-test", Priority: domain.PriorityMedium,
		PickupNotBefore: &future,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if q.BufferDepth() != 0 {
		t.Fatalf("buffer depth before wake: %d want 0", q.BufferDepth())
	}

	deadline := time.After(2 * time.Second)
	for q.BufferDepth() == 0 {
		select {
		case <-deadline:
			t.Fatal("timed out waiting for pickup wake to enqueue")
		case <-time.After(5 * time.Millisecond):
		}
	}
	got, err := q.Receive(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != tk.ID {
		t.Fatalf("task id %q want %q", got.ID, tk.ID)
	}
	q.AckAfterRecv(got.ID)
}

func TestPickupWakeScheduler_CancelPreventsWake(t *testing.T) {
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := NewMemoryQueue(32)
	st.SetReadyTaskNotifier(q)
	w := NewPickupWakeScheduler(st, q)
	st.SetPickupWake(w)
	defer w.Stop()

	future := time.Now().UTC().Add(200 * time.Millisecond)
	tk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "cancel-test", Priority: domain.PriorityMedium,
		PickupNotBefore: &future,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	w.Cancel(tk.ID)

	select {
	case got := <-q.Recv():
		t.Fatalf("unexpected enqueue for cancelled task: %v", got.ID)
	case <-time.After(300 * time.Millisecond):
	}
}
