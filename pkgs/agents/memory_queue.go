package agents

import (
	"context"
	"errors"
	"log/slog"
	"sync"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const agentsLogCmd = "taskapi"

// MemoryQueue is a bounded FIFO of full domain.Task snapshots for in-process agent consumers.
// It tracks task ids currently buffered so reconciliation can skip ids already present.
type MemoryQueue struct {
	mu      sync.Mutex
	pending map[string]struct{}
	ch      chan domain.Task
}

// NewMemoryQueue returns a queue that holds at most cap tasks without blocking producers.
// cap must be positive.
func NewMemoryQueue(cap int) *MemoryQueue {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.NewMemoryQueue", "cap", cap)
	if cap <= 0 {
		panic("agents: NewMemoryQueue cap must be positive")
	}
	return &MemoryQueue{
		ch:      make(chan domain.Task, cap),
		pending: make(map[string]struct{}),
	}
}

// Recv exposes the receive side for a single consumer (or fan out in your own goroutines).
// After reading each task, call AckAfterRecv with the task id so reconciliation
// can treat the buffer as drained for that id.
func (q *MemoryQueue) Recv() <-chan domain.Task {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.Recv")
	if q == nil {
		return nil
	}
	return q.ch
}

// AckAfterRecv drops id from the queue's pending set. Call once after consuming a task from Recv.
func (q *MemoryQueue) AckAfterRecv(id string) {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.AckAfterRecv", "task_id", id)
	if q == nil || id == "" {
		return
	}
	q.mu.Lock()
	delete(q.pending, id)
	q.mu.Unlock()
}

// Receive waits for the next task, removes it from the pending set, and returns it.
func (q *MemoryQueue) Receive(ctx context.Context) (domain.Task, error) {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.Receive")
	if q == nil {
		return domain.Task{}, errors.New("agents: nil MemoryQueue")
	}
	select {
	case t := <-q.ch:
		q.mu.Lock()
		delete(q.pending, t.ID)
		q.mu.Unlock()
		return t, nil
	case <-ctx.Done():
		return domain.Task{}, ctx.Err()
	}
}

// NotifyUserTaskCreated implements UserTaskCreatedNotifier. It never blocks: if the buffer is
// full it returns ErrQueueFull. If the task id is already pending it returns ErrAlreadyQueued.
func (q *MemoryQueue) NotifyUserTaskCreated(ctx context.Context, task domain.Task) error {
	_ = ctx
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.NotifyUserTaskCreated", "task_id", task.ID)
	if q == nil {
		return nil
	}
	if task.ID == "" {
		return errors.New("agents: task id required")
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if _, exists := q.pending[task.ID]; exists {
		return ErrAlreadyQueued
	}
	select {
	case q.ch <- task:
		q.pending[task.ID] = struct{}{}
		return nil
	default:
		return ErrQueueFull
	}
}
