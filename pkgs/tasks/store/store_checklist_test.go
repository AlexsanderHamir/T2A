package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
)

func TestStore_SetChecklistItemDone_rejects_user_actor(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "criterion", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	err = s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_SetChecklistItemDone_allows_agent(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "criterion", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetChecklistItemDone(ctx, tsk.ID, it.ID, true, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || !items[0].Done || items[0].ID != it.ID {
		t.Fatalf("checklist: %+v", items)
	}
}
