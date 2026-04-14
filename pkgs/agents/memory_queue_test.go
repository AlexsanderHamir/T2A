package agents

import (
	"context"
	"errors"
	"testing"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

func TestMemoryQueue_NotifyReadyTask_rejectsCancelledContext(t *testing.T) {
	q := NewMemoryQueue(2)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	t1 := domain.Task{ID: "11111111-1111-4111-8111-111111111111", Title: "a", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	err := q.NotifyReadyTask(ctx, t1)
	if err == nil || !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v want context.Canceled", err)
	}
}

func TestMemoryQueue_deliversTask(t *testing.T) {
	q := NewMemoryQueue(2)
	t1 := domain.Task{ID: "11111111-1111-4111-8111-111111111111", Title: "a", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	if err := q.NotifyUserTaskCreated(context.Background(), t1); err != nil {
		t.Fatal(err)
	}
	got := <-q.Recv()
	q.AckAfterRecv(got.ID)
	if got.ID != t1.ID || got.Title != t1.Title {
		t.Fatalf("got %+v want %+v", got, t1)
	}
}

func TestMemoryQueue_ErrAlreadyQueued(t *testing.T) {
	q := NewMemoryQueue(2)
	t1 := domain.Task{ID: "11111111-1111-4111-8111-111111111111", Title: "a", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	if err := q.NotifyUserTaskCreated(context.Background(), t1); err != nil {
		t.Fatal(err)
	}
	if err := q.NotifyUserTaskCreated(context.Background(), t1); !errors.Is(err, ErrAlreadyQueued) {
		t.Fatalf("want ErrAlreadyQueued got %v", err)
	}
}

func TestMemoryQueue_BufferCap_and_BufferDepth(t *testing.T) {
	q := NewMemoryQueue(3)
	if q.BufferCap() != 3 {
		t.Fatalf("BufferCap: got %d", q.BufferCap())
	}
	if q.BufferDepth() != 0 {
		t.Fatalf("empty depth: got %d", q.BufferDepth())
	}
	t1 := domain.Task{ID: "11111111-1111-4111-8111-111111111111", Title: "a", Priority: domain.PriorityMedium, TaskType: domain.TaskTypeGeneral}
	if err := q.NotifyUserTaskCreated(context.Background(), t1); err != nil {
		t.Fatal(err)
	}
	if q.BufferDepth() != 1 {
		t.Fatalf("after enqueue depth: got %d", q.BufferDepth())
	}
	<-q.Recv()
	q.AckAfterRecv(t1.ID)
	if q.BufferDepth() != 0 {
		t.Fatalf("after ack depth: got %d", q.BufferDepth())
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
	got1 := <-q.Recv()
	q.AckAfterRecv(got1.ID)
	if err := q.NotifyUserTaskCreated(context.Background(), t2); err != nil {
		t.Fatal(err)
	}
	got := <-q.Recv()
	q.AckAfterRecv(got.ID)
	if got.ID != t2.ID {
		t.Fatalf("got id %q", got.ID)
	}
}
