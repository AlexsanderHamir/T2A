package store

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/internal/testdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/postgres"
	"gorm.io/gorm"
)

func strPtr(s string) *string { return &s }

func TestStore_Create_rejects_empty_title(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "   "}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_status(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	st := domain.Status("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Status: st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_priority(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	pr := domain.Priority("nope")
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok", Priority: pr}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_rejects_invalid_actor(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Create(context.Background(), CreateTaskInput{Title: "ok"}, domain.Actor("system"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_uses_explicit_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	id := "custom-id-1"
	got, err := s.Create(context.Background(), CreateTaskInput{ID: id, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Fatalf("id %q", got.ID)
	}
}

func TestStore_Get_not_found(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "00000000-0000-0000-0000-000000000099")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Get_rejects_empty_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	_, err := s.Get(context.Background(), "  ")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_List_pagination_and_limit_cap(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	for i := range 5 {
		title := string(rune('a' + i))
		if _, err := s.Create(ctx, CreateTaskInput{Title: title}, domain.ActorUser); err != nil {
			t.Fatal(err)
		}
	}

	out, err := s.List(ctx, 2, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("page1 len %d", len(out))
	}

	out2, err := s.List(ctx, 2, 2)
	if err != nil {
		t.Fatal(err)
	}
	if len(out2) != 2 {
		t.Fatalf("page2 len %d", len(out2))
	}

	all, err := s.List(ctx, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 5 {
		t.Fatalf("limit 0 normalized len %d", len(all))
	}

	capped, err := s.List(ctx, 500, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(capped) != 5 {
		t.Fatalf("over-limit cap: got %d tasks", len(capped))
	}
}

func TestStore_Update_rejects_no_fields(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_rejects_empty_title_patch(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("  ")}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_changes_status_and_prompt(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "x"}, domain.ActorUser)
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
	s := NewStore(testdb.OpenSQLite(t))
	st := domain.StatusRunning
	_, err := s.Update(context.Background(), "00000000-0000-0000-0000-000000000088", UpdateTaskInput{Status: &st}, domain.ActorUser)
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_not_found(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	err := s.Delete(context.Background(), "00000000-0000-0000-0000-000000000077")
	if !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("got %v want ErrNotFound", err)
	}
}

func TestStore_Delete_rejects_empty_id(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	err := s.Delete(context.Background(), "")
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Create_duplicate_primary_key_fails(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	id := "dup"
	if _, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "a"}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	_, err := s.Create(ctx, CreateTaskInput{ID: id, Title: "b"}, domain.ActorUser)
	if err == nil {
		t.Fatal("expected error on duplicate id")
	}
	if errors.Is(err, domain.ErrInvalidInput) || errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unexpected sentinel: %v", err)
	}
}

func TestStore_events_recorded_for_create_and_title_change(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "first"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Update(ctx, tsk.ID, UpdateTaskInput{Title: strPtr("second")}, domain.ActorAgent); err != nil {
		t.Fatal(err)
	}
	var n int64
	if err := db.Model(&domain.TaskEvent{}).Where("task_id = ?", tsk.ID).Count(&n).Error; err != nil {
		t.Fatal(err)
	}
	if n < 2 {
		t.Fatalf("task_events rows %d want >= 2", n)
	}
}

func TestStore_Update_rejects_invalid_actor(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	st := domain.StatusRunning
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Status: &st}, domain.Actor("nope"))
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestStore_Update_rejects_invalid_status_value(t *testing.T) {
	s := NewStore(testdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	bad := domain.Status("invalid")
	_, err = s.Update(ctx, tsk.ID, UpdateTaskInput{Status: &bad}, domain.ActorUser)
	if !errors.Is(err, domain.ErrInvalidInput) {
		t.Fatalf("got %v want ErrInvalidInput", err)
	}
}

func TestMigrate_second_call_succeeds(t *testing.T) {
	db := testdb.OpenSQLite(t)
	if err := postgres.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
}

func TestNewStore_roundTrip(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	in, err := s.Create(ctx, CreateTaskInput{Title: "r"}, domain.ActorUser)
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
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	got, err := s.List(context.Background(), 10, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("len %d", len(got))
	}
}

func TestStore_Delete_cascades_events(t *testing.T) {
	db := testdb.OpenSQLite(t)
	s := NewStore(db)
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Title: "x"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Delete(ctx, tsk.ID); err != nil {
		t.Fatal(err)
	}
	err = db.Where("task_id = ?", tsk.ID).First(&domain.TaskEvent{}).Error
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		t.Fatalf("expected events removed, got err=%v", err)
	}
}
