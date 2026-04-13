package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestListReadyTaskQueueCandidates_ordersOldestCreatedFirst(t *testing.T) {
	ctx := context.Background()
	s := NewStore(tasktestdb.OpenSQLite(t))
	// Lexicographically smaller id, but created second — should not win over older task.
	idOlder := "bbbbbbbb-bbbb-4bbb-8bbb-bbbbbbbbbbbb"
	idNewer := "11111111-1111-4111-8111-111111111111"
	if _, err := s.Create(ctx, CreateTaskInput{ID: idOlder, Title: "older", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, CreateTaskInput{ID: idNewer, Title: "newer", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	page1, err := s.ListReadyTaskQueueCandidates(ctx, 1, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 1 || page1[0].Task.ID != idOlder {
		t.Fatalf("first page got %+v want id %q", page1, idOlder)
	}
	cur := &ReadyTaskQueueCursor{
		AfterTaskCreatedAt: page1[0].TaskCreatedAt,
		AfterTaskID:        page1[0].Task.ID,
		AfterEventRowID:    page1[0].EventRowID,
	}
	page2, err := s.ListReadyTaskQueueCandidates(ctx, 50, cur)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].Task.ID != idNewer {
		t.Fatalf("second page got %+v want id %q", page2, idNewer)
	}
}

func TestListReadyTaskQueueCandidates_nilCursorInvalidPartial(t *testing.T) {
	ctx := context.Background()
	s := NewStore(tasktestdb.OpenSQLite(t))
	_, err := s.ListReadyTaskQueueCandidates(ctx, 10, &ReadyTaskQueueCursor{})
	if err == nil {
		t.Fatal("expected error for cursor with empty id")
	}
}
