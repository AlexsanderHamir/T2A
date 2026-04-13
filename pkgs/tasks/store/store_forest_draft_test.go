package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestStore_ListRootForest_empty_nonNilSlice(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	got, hasMore, err := s.ListRootForest(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Fatal("unexpected hasMore")
	}
	if got == nil {
		t.Fatal("want empty non-nil slice so JSON encodes as [] not null")
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_ListRootForest_nested(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	p, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "root"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := p.ID
	_, err = s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "kid", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	forest, hasMore, err := s.ListRootForest(ctx, 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if hasMore {
		t.Fatal("unexpected hasMore")
	}
	if len(forest) != 1 {
		t.Fatalf("roots %d", len(forest))
	}
	if len(forest[0].Children) != 1 {
		t.Fatalf("children %d", len(forest[0].Children))
	}
	if forest[0].Children[0].Title != "kid" {
		t.Fatalf("child title %q", forest[0].Children[0].Title)
	}
}

func TestStore_ListRootForest_hasMore_and_keyset(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	ids := []string{
		"10000000-0000-4000-8000-000000000001",
		"10000000-0000-4000-8000-000000000002",
		"10000000-0000-4000-8000-000000000003",
	}
	for _, id := range ids {
		if _, err := s.Create(ctx, CreateTaskInput{ID: id, Priority: domain.PriorityMedium, Title: "r"}, domain.ActorUser); err != nil {
			t.Fatal(err)
		}
	}
	got, hasMore, err := s.ListRootForest(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if !hasMore || len(got) != 2 || got[0].ID != ids[0] || got[1].ID != ids[1] {
		t.Fatalf("page1: len=%d hasMore=%v", len(got), hasMore)
	}
	got2, hasMore2, err := s.ListRootForestAfter(ctx, 2, ids[1])
	if err != nil {
		t.Fatal(err)
	}
	if hasMore2 || len(got2) != 1 || got2[0].ID != ids[2] {
		t.Fatalf("page2: len=%d hasMore=%v", len(got2), hasMore2)
	}
}

func TestStore_Create_child_appends_subtask_event_on_parent(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	child, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "kid", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	chEv, err := s.ListTaskEvents(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(chEv) != 1 || chEv[0].Type != domain.EventTaskCreated {
		t.Fatalf("child events: %+v", chEv)
	}
	pEv, err := s.ListTaskEvents(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pEv) != 2 || pEv[0].Type != domain.EventTaskCreated || pEv[1].Type != domain.EventSubtaskAdded {
		t.Fatalf("parent events: %+v", pEv)
	}
}

func TestStore_Update_checklist_inherit_change_appends_event(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	child, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "c", ParentID: &pid, ChecklistInherit: false}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	inherit := true
	if _, err := s.Update(ctx, child.ID, UpdateTaskInput{ChecklistInherit: &inherit}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	evs, err := s.ListTaskEvents(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range evs {
		if e.Type == domain.EventChecklistInheritChanged {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatal("expected checklist_inherit_changed event")
	}
}

func TestStore_Ping_ok(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	if err := s.Ping(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStore_Ready_ok(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	if err := s.Ready(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestStore_Ready_fails_when_db_closed(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatal(err)
	}
	if err := sqlDB.Close(); err != nil {
		t.Fatal(err)
	}
	if err := s.Ready(context.Background()); err == nil {
		t.Fatal("expected error after close")
	}
}

func TestStore_EvaluateDraftTask_persists_multiple_evaluations(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()

	in := EvaluateDraftTaskInput{
		DraftID:       "draft-abc",
		Title:         "Harden task event pagination",
		InitialPrompt: "Add cursor tests for before_seq and after_seq",
		Priority:      domain.PriorityHigh,
	}
	a, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	b, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if a.EvaluationID == b.EvaluationID {
		t.Fatal("expected unique evaluation ids")
	}
	rows, err := s.ListDraftEvaluations(ctx, "draft-abc", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
}

func TestStore_Create_attaches_draft_evaluations_to_task(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	in := EvaluateDraftTaskInput{
		DraftID:       "draft-link-1",
		Title:         "Link draft evals",
		InitialPrompt: "Persist and bind evaluations on create",
		Priority:      domain.PriorityMedium,
	}
	if _, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EvaluateDraftTask(ctx, in, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	task, err := s.Create(ctx, CreateTaskInput{
		Title:    "Final task",
		Priority: domain.PriorityMedium,
		DraftID:  "draft-link-1",
	}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := s.ListDraftEvaluations(ctx, "draft-link-1", 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	for _, row := range rows {
		if row.TaskID == nil || *row.TaskID != task.ID {
			t.Fatalf("expected task_id %q, got %#v", task.ID, row.TaskID)
		}
	}
}

func TestStore_DraftCRUD_roundtrip(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	saved, err := s.SaveDraft(ctx, "", "My draft", []byte(`{"title":"A"}`))
	if err != nil {
		t.Fatal(err)
	}
	if saved.ID == "" {
		t.Fatal("expected generated draft id")
	}
	got, err := s.GetDraft(ctx, saved.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "My draft" {
		t.Fatalf("name %q", got.Name)
	}
	list, err := s.ListDrafts(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].ID != saved.ID {
		t.Fatalf("list %#v", list)
	}
	if err := s.DeleteDraft(ctx, saved.ID); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetDraft(ctx, saved.ID)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("want not found, got %v", err)
	}
}
