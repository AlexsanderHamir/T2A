package store

import (
	"context"
	"testing"
	"time"

	"github.com/AlexsanderHamir/Hamix/internal/tasktestdb"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/scheduling"
	"github.com/AlexsanderHamir/Hamix/pkgs/tasks/store/internal/tasks"
)

// I3 contract gate: scheduling.EvaluateWorkerReadiness must match ListQueueCandidates.
func TestSchedulingParity_GoReadinessMatchesListQueueCandidates(t *testing.T) {
	t.Parallel()
	db := tasktestdb.OpenSQLite(t)
	ctx := context.Background()
	s := NewStore(db)
	now := time.Now().UTC()

	dep, err := s.Create(ctx, CreateTaskInput{
		Title: "dep", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := s.Create(ctx, CreateTaskInput{
		Title: "blocked", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddTaskDependency(ctx, blocked.ID, dep.ID, domain.DependencySatisfiesDone); err != nil {
		t.Fatal(err)
	}

	heldGate := &domain.TaskGate{Kind: domain.GateKindManualApproval, Status: domain.GateStatusActive}
	held, err := s.Create(ctx, CreateTaskInput{
		Title: "held", Status: domain.StatusReady, Priority: domain.PriorityMedium, Gate: heldGate,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	future := now.Add(2 * time.Hour)
	if _, err := s.Create(ctx, CreateTaskInput{
		Title: "deferred", Status: domain.StatusReady, Priority: domain.PriorityMedium,
		PickupNotBefore: &future,
	}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}

	eligible, err := s.Create(ctx, CreateTaskInput{
		Title: "eligible", Status: domain.StatusReady, Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	candidateIDs := map[string]struct{}{}
	cands, err := s.ListReadyTaskQueueCandidates(ctx, 100, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range cands {
		candidateIDs[c.Task.ID] = struct{}{}
	}

	allTasks, err := s.ListFlat(ctx, 100, 0, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range allTasks {
		if allTasks[i].Status != domain.StatusReady {
			continue
		}
		task := &allTasks[i]
		depsMet, err := tasks.DependenciesSatisfied(ctx, db, task.ID)
		if err != nil {
			t.Fatal(err)
		}
		r := scheduling.EvaluateWorkerReadiness(task, now, depsMet)
		_, inSQL := candidateIDs[task.ID]
		if r.Ready != inSQL {
			t.Fatalf("task %q: Go ready=%v (pred=%s) SQL candidate=%v", task.ID, r.Ready, r.FailedPredicate, inSQL)
		}
	}

	if _, ok := candidateIDs[eligible.ID]; !ok {
		t.Fatalf("eligible task %q should be SQL candidate", eligible.ID)
	}
	if _, ok := candidateIDs[blocked.ID]; ok {
		t.Fatalf("blocked task %q should not be SQL candidate", blocked.ID)
	}
	if _, ok := candidateIDs[held.ID]; ok {
		t.Fatalf("held task %q should not be SQL candidate", held.ID)
	}
}
