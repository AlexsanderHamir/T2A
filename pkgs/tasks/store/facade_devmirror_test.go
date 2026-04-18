package store

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestStore_ApplyDevTaskRowMirror_status(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	tsk, err := s.Create(ctx, CreateTaskInput{Priority: domain.PriorityMedium, Title: "t"}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(map[string]string{"from": "ready", "to": "running"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ApplyDevTaskRowMirror(ctx, tsk.ID, domain.EventStatusChanged, data); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(ctx, tsk.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != domain.StatusRunning {
		t.Fatalf("status %q", got.Status)
	}
}

func TestStore_ListDevsimTasks_like(t *testing.T) {
	s := NewStore(tasktestdb.OpenSQLite(t))
	ctx := context.Background()
	if _, err := s.Create(ctx, CreateTaskInput{ID: "t2a-devsim-aa", Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, CreateTaskInput{ID: "other-id", Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	got, err := s.ListDevsimTasks(ctx, "t2a-devsim-%")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != "t2a-devsim-aa" {
		t.Fatalf("got %+v", got)
	}
}
