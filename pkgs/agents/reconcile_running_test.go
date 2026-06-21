package agents_test

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/agents"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store"
)

func TestReconcileRunningTasksNotQueued_enqueuesOpenCycle(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	st := store.NewStore(tasktestdb.OpenSQLite(t))
	q := agents.NewMemoryQueue(8)

	tsk, err := st.Create(ctx, store.CreateTaskInput{
		Title: "resume-me", InitialPrompt: "work", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	running := domain.StatusRunning
	if _, err := st.Update(ctx, tsk.ID, store.UpdateTaskInput{Status: &running}, domain.ActorAgent); err != nil {
		t.Fatalf("update running: %v", err)
	}
	if _, err := st.StartCycle(ctx, store.StartCycleInput{TaskID: tsk.ID, TriggeredBy: domain.ActorAgent}); err != nil {
		t.Fatalf("start cycle: %v", err)
	}

	res, err := agents.ReconcileRunningTasksNotQueued(ctx, st, q)
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if res.Scanned != 1 || res.Enqueued != 1 {
		t.Fatalf("reconcile result = %+v, want scanned=1 enqueued=1", res)
	}

	res2, err := agents.ReconcileRunningTasksNotQueued(ctx, st, q)
	if err != nil {
		t.Fatalf("second reconcile: %v", err)
	}
	if res2.Enqueued != 0 || res2.SkippedAlreadyQueued != 1 {
		t.Fatalf("second reconcile = %+v, want skipped already queued", res2)
	}
}
