package agents

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"sync"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const agentsLogCmd = "taskapi"

// MemoryQueue is a bounded FIFO of full domain.Task snapshots for in-process agent consumers.
// It tracks task ids currently buffered so reconciliation can skip ids already present.
type MemoryQueue struct {
	mu      sync.Mutex
	bufCap  int
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
		bufCap:  cap,
		ch:      make(chan domain.Task, cap),
		pending: make(map[string]struct{}),
	}
}

// BufferCap returns the configured channel buffer size (max tasks without blocking producers).
func (q *MemoryQueue) BufferCap() int {
	if q == nil {
		return 0
	}
	return q.bufCap
}

// BufferDepth returns how many tasks are currently queued (buffered channel length).
func (q *MemoryQueue) BufferDepth() int {
	if q == nil {
		return 0
	}
	return len(q.ch)
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

// tryEnqueue adds task to the buffer when there is capacity and the id is not already pending.
func (q *MemoryQueue) tryEnqueue(task domain.Task) error {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.tryEnqueue", "task_id", task.ID)
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

// NotifyUserTaskCreated implements UserTaskCreatedNotifier. It never blocks: if the buffer is
// full it returns ErrQueueFull. If the task id is already pending it returns ErrAlreadyQueued.
func (q *MemoryQueue) NotifyUserTaskCreated(ctx context.Context, task domain.Task) error {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.NotifyUserTaskCreated", "task_id", task.ID)
	if err := notifyContextErr(ctx); err != nil {
		return err
	}
	return q.tryEnqueue(task)
}

// NotifyReadyTask is the hook used by pkgs/tasks/store.ReadyTaskNotifier wiring. Same enqueue semantics as NotifyUserTaskCreated.
func (q *MemoryQueue) NotifyReadyTask(ctx context.Context, task domain.Task) error {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.NotifyReadyTask", "task_id", task.ID)
	if err := notifyContextErr(ctx); err != nil {
		return err
	}
	return q.tryEnqueue(task)
}

func notifyContextErr(ctx context.Context) error {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.notifyContextErr")
	if ctx == nil {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("agents: context done before notify: %w", err)
	}
	return nil
}
