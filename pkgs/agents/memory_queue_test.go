package agents

import (
	"context"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestMemoryQueue_deliversTask(t *testing.T) {
	q := NewMemoryQueue(2)
	t1 := domain.Task{ID: "11111111-1111-4111-8111-111111111111", Title: "a", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	if err := q.NotifyUserTaskCreated(context.Background(), t1); err != nil {
		t.Fatal(err)
	}
	got := <-q.Recv()
	if got.ID != t1.ID || got.Title != t1.Title {
		t.Fatalf("got %+v want %+v", got, t1)
	}
}

func TestMemoryQueue_fullReturnsErrQueueFull(t *testing.T) {
	q := NewMemoryQueue(1)
	t1 := domain.Task{ID: "11111111-1111-4111-8111-111111111111", Title: "a", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	t2 := domain.Task{ID: "22222222-2222-4222-8222-222222222222", Title: "b", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	if err := q.NotifyUserTaskCreated(context.Background(), t1); err != nil {
		t.Fatal(err)
	}
	if err := q.NotifyUserTaskCreated(context.Background(), t2); err != ErrQueueFull {
		t.Fatalf("want ErrQueueFull got %v", err)
	}
	<-q.Recv()
	if err := q.NotifyUserTaskCreated(context.Background(), t2); err != nil {
		t.Fatal(err)
	}
	got := <-q.Recv()
	if got.ID != t2.ID {
		t.Fatalf("got id %q", got.ID)
	}
}
