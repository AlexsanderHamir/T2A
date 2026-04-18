package store

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/internal/tasktestdb"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// --- Agent-queue path: ListReadyTaskQueueCandidates ----------------------

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

// --- User-facing path: ListReadyTasksUserCreated -------------------------

func TestListReadyTasksUserCreated_filtersActorAndStatus(t *testing.T) {
	ctx := context.Background()
	s := NewStore(tasktestdb.OpenSQLite(t))

	u1, err := s.Create(ctx, CreateTaskInput{Title: "user ready", Priority: domain.PriorityMedium}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}
	_, err = s.Create(ctx, CreateTaskInput{Title: "agent ready", Priority: domain.PriorityMedium}, domain.ActorAgent)
	if err != nil {
		t.Fatal(err)
	}
	u3, err := s.Create(ctx, CreateTaskInput{Title: "user running", Priority: domain.PriorityMedium, Status: domain.StatusRunning}, domain.ActorUser)
	if err != nil {
		t.Fatal(err)
	}

	got, err := s.ListReadyTasksUserCreated(ctx, 50, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ID != u1.ID {
		t.Fatalf("got %+v want single user-ready id %s", got, u1.ID)
	}
	if got[0].Status != domain.StatusReady {
		t.Fatalf("status %s", got[0].Status)
	}
	_ = u3
}

func TestListReadyTasksUserCreated_paginationAfterID(t *testing.T) {
	ctx := context.Background()
	s := NewStore(tasktestdb.OpenSQLite(t))

	const id1 = "11111111-1111-4111-8111-111111111111"
	const id2 = "22222222-2222-4222-8222-222222222222"
	if _, err := s.Create(ctx, CreateTaskInput{ID: id1, Title: "a", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Create(ctx, CreateTaskInput{ID: id2, Title: "b", Priority: domain.PriorityMedium}, domain.ActorUser); err != nil {
		t.Fatal(err)
	}
	page1, err := s.ListReadyTasksUserCreated(ctx, 1, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 1 || page1[0].ID != id1 {
		t.Fatalf("page1 %+v", page1)
	}
	page2, err := s.ListReadyTasksUserCreated(ctx, 50, page1[0].ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 1 || page2[0].ID != id2 {
		t.Fatalf("page2 %+v want %s", page2, id2)
	}
}
