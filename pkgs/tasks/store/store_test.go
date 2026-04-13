package store

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"gorm.io/gorm"
)

func strPtr(s string) *string { return &s }

func TestStore_Create_rejects_empty_title(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "   "}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_status(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	st := domain.Status("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "ok", Status: st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_missing_priority(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok"}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_priority(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	pr := domain.Priority("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: pr}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_defaults_task_type_to_general(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	got, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.TaskType != domain.TaskTypeGeneral {
		t.Fatalf("task type %q", got.TaskType)
	}
}

func TestStore_Create_rejects_invalid_task_type(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	tt := domain.TaskType("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: domain.PriorityMedium, TaskType: tt}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_actor(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Priority: domain.PriorityMedium, Title: "ok"}, domain.Actor("system"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_uses_explicit_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	id := "custom-id-1"
	got, err := s.Create(context.Background(), CreateTaskInput{ID: id, Title: "t", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("id %q", got.ID)
	}
}

func TestStore_Get_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000099")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Get_rejects_empty_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "  ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_GetTaskTree_rejects_chain_deeper_than_max(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	root, err := s.Create(ctx, CreateTaskInput{Title: "root", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := root.ID
	for i := 0; i < MaxTaskTreeDepth; i++ {
		child, err := s.Create(ctx, CreateTaskInput{Title: fmt.Sprintf("d%d", i), Priority: domain.PriorityMedium, ParentID: &pid}, domain.ActorUser)
		if err != nil {
			t.Fatal(err)
		}
		pid = child.ID
	}
	if _, err := s.GetTaskTree(ctx, root.ID); err != nil {
		t.Fatalf("tree at max depth should succeed: %v", err)
	}
	if _, err := s.Create(ctx, CreateTaskInput{Title: "too-deep", Priority: domain.PriorityMedium, ParentID: &pid}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	_, err = s.GetTaskTree(ctx, root.ID)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_List_pagination_and_limit_cap(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	for i := range 5 {
		title := string(rune('a' + i))
		if _, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: title}, domain.ActorUser); err != nil {
			t.Fatal(err)
		}
	}

	out, err := s.ListFlat(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("page1 len %d", len(out))
	}

	out2, err := s.ListFlat(ctx, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) != 2 {
		t.Fatalf("page2 len %d", len(out2))
	}

	all, err := s.ListFlat(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("limit 0 normalized len %d", len(all))
	}

	capped, err := s.ListFlat(ctx, 500, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 5 {
		t.Fatalf("over-limit cap: got %d tasks", len(capped))
	}
}

func TestStore_Update_rejects_no_fields(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_rejects_empty_title_patch(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("  ")}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_changes_status_and_prompt(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	st := domain.StatusRunning
	pr := domain.PriorityHigh
	got, err := s.Update(ctx, tsk.ID, UpdateTaskInput{
		InitialPrompt: strPtr("p1"),
		Status:        &st,
		Priority:      &pr,
	}, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusRunning || got.Priority != domain.PriorityHigh || got.InitialPrompt != "p1" {
		t.Fatalf("got %+v", got)
	}
}

func TestStore_Update_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	st := domain.StatusRunning
	_, err := s.Update(context.Background(), "00000000-0000-0000-0000-000000000088", UpdateTaskInput{Status: &st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_not_found(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000077", domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_rejects_empty_id(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.Delete(context.Background(), "", domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_duplicate_primary_key_fails(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	id := "dup"
	if _, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	_, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err == nil {
		t.Fatal("expected error on duplicate id")
	}
	if !errors.Is(err, domain.ErrConflict) {
		t.Fatalf("want ErrConflict, got %v", err)
	}
}

func TestMigrate_second_call_succeeds(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	if err := postgres.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

func TestNewStore_roundTrip(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	in, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "r"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	out, err := s.Get(ctx, in.ID)
	if err != nil {
		t.Fatal(err)
	}
	if out.Title != "r" {
		t.Fatalf("title %q", out.Title)
	}
}

func TestStore_List_empty_table(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	got, err := s.ListFlat(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_Delete_cascades_events(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Delete(ctx, tsk.ID, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	err = db.Where("task_id = ?", tsk.ID).First(&domain.TaskEvent{}).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected events removed, got err=%v", err)
	}
}

func TestStore_Update_done_blockedWhenChildNotDone(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	_, err = s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "c", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	done := domain.StatusDone
	_, err = s.Update(ctx, parent.ID, UpdateTaskInput{Status: &done}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Delete_blockedWhenChildrenExist(t *testing.T) {
	db := tasktestdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	parent, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "p"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	pid := parent.ID
	_, err = s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "c", ParentID: &pid}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Delete(ctx, parent.ID, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Delete_child_appends_subtask_removed_on_parent(t *testing.T) {
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
	parentNotify, err := s.Delete(ctx, child.ID, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if parentNotify != parent.ID {
		t.Fatalf("notify parent %q want %q", parentNotify, parent.ID)
	}
	pEv, err := s.ListTaskEvents(ctx, parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, e := range pEv {
		if e.Type == domain.EventSubtaskRemoved {
			saw = true
			break
		}
	}
	if !saw {
		t.Fatalf("parent events: want subtask_removed, got %#v", pEv)
	}
}
