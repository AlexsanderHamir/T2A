package store_test

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/store"
)

func TestStore_PendingRetry_setAndAgentPickupConsumes(t *testing.T) {
	ctx := context.Background()
	db := tasktestdb.OpenSQLite(t)
	s := store.NewStore(db)

	tsk, err := s.Create(ctx, store.CreateTaskInput{
		Title: "retry-intent", InitialPrompt: "p", Priority: domain.PriorityMedium, Status: domain.StatusFailed,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	intent := &domain.PendingRetry{Mode: domain.RetryFresh, ParentCycleID: "parent-cycle-id"}
	ready := domain.StatusReady
	got, err := s.Update(ctx, tsk.ID, store.UpdateTaskInput{
		Status:       &ready,
		PendingRetry: intent,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.PendingRetry == nil || got.PendingRetry.Mode != domain.RetryFresh {
		t.Fatalf("pending_retry: %+v", got.PendingRetry)
	}

	pickup, err := s.AgentPickup(ctx, tsk.ID, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if pickup.Task.Status != domain.StatusRunning {
		t.Fatalf("status %q want running", pickup.Task.Status)
	}
	if pickup.Task.PendingRetry != nil {
		t.Fatal("pending_retry should be cleared on pickup")
	}
	if pickup.ConsumedRetry == nil || pickup.ConsumedRetry.ParentCycleID != intent.ParentCycleID {
		t.Fatalf("consumed: %+v", pickup.ConsumedRetry)
	}

	after, err := s.Get(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if after.PendingRetry != nil {
		t.Fatal("pending_retry persisted after pickup")
	}
}

func TestStore_AgentPickup_rejectsNonReady(t *testing.T) {
	ctx := context.Background()
	db := tasktestdb.OpenSQLite(t)
	s := store.NewStore(db)

	tsk, err := s.Create(ctx, store.CreateTaskInput{
		Title: "not-ready", InitialPrompt: "p", Priority: domain.PriorityMedium, Status: domain.StatusFailed,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.AgentPickup(ctx, tsk.ID, domain.ActorAgent)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_pendingRetrySetAndClearConflict(t *testing.T) {
	ctx := context.Background()
	db := tasktestdb.OpenSQLite(t)
	s := store.NewStore(db)

	tsk, err := s.Create(ctx, store.CreateTaskInput{
		Title: "conflict", InitialPrompt: "p", Priority: domain.PriorityMedium,
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	intent := &domain.PendingRetry{Mode: domain.RetryResume, ParentCycleID: "c1"}
	_, err = s.Update(ctx, tsk.ID, store.UpdateTaskInput{
		PendingRetry:      intent,
		ClearPendingRetry: true,
	}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}
