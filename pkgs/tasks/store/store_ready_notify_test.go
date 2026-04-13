package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

type spyReadyNotifier struct {
	calls int
	last  string
}

func (s *spyReadyNotifier) NotifyReadyTask(ctx context.Context, task domain.Task) error {
	s.calls++
	s.last = task.ID
	return nil
}

func TestSetReadyTaskNotifier_CreateReady(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	if _, err := st.Create(ctx, CreateTaskInput{Title: "x", Priority: domain.PriorityMedium}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	if n.calls != 1 {
		t.Fatalf("notifier calls %d want 1", n.calls)
	}
}

func TestSetReadyTaskNotifier_CreateNonReady(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	if _, err := st.Create(ctx, CreateTaskInput{Title: "x", Priority: domain.PriorityMedium, Status: domain.StatusRunning}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 0 {
		t.Fatalf("notifier calls %d want 0", n.calls)
	}
}

func TestSetReadyTaskNotifier_UpdateTransitionToReady(t *testing.T) {
	ctx := context.Background()
	st := NewStore(tasktestdb.OpenSQLite(t))
	var n spyReadyNotifier
	st.SetReadyTaskNotifier(&n)
	tk, err := st.Create(ctx, CreateTaskInput{Title: "x", Priority: domain.PriorityMedium, Status: domain.StatusRunning}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	n.calls = 0
	ready := domain.StatusReady
	if _, err := st.Update(ctx, tk.ID, UpdateTaskInput{Status: &ready}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if n.calls != 1 || n.last != tk.ID {
		t.Fatalf("calls=%d last=%q", n.calls, n.last)
	}
}
