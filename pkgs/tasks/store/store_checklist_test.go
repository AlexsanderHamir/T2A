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
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
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
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
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

func TestStore_UpdateChecklistItemText_updates_row(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "before", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateChecklistItemText(ctx, tsk.ID, it.ID, "after", domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].Text != "after" {
		t.Fatalf("checklist: %+v", items)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range evs {
		if e.Type == domain.EventChecklistItemUpdated {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected checklist_item_updated event")
	}
}

func TestStore_UpdateChecklistItemText_rejects_checklist_inherit(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, parent.ID, "c", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.Create(ctx, CreateTaskInput{Title: "c", ParentID: &parent.ID, ChecklistInherit: true, Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	err = s.UpdateChecklistItemText(ctx, child.ID, it.ID, "nope", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_DeleteChecklistItem_appends_removed_event(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.AddChecklistItem(ctx, tsk.ID, "gone", domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteChecklistItem(ctx, tsk.ID, it.ID, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	items, err := s.ListChecklistForSubject(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 0 {
		t.Fatalf("checklist: %+v", items)
	}
	evs, err := s.ListTaskEvents(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range evs {
		if e.Type == domain.EventChecklistItemRemoved {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected checklist_item_removed event")
	}
}
