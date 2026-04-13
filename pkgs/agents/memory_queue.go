package agents

import (
	"context"
	"log/slog"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

const agentsLogCmd = "taskapi"

// MemoryQueue is a bounded FIFO of full [domain.Task] snapshots for in-process agent consumers.
type MemoryQueue struct {
	ch chan domain.Task
}

// NewMemoryQueue returns a queue that holds at most cap tasks without blocking producers.
// cap must be positive.
func NewMemoryQueue(cap int) *MemoryQueue {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.NewMemoryQueue", "cap", cap)
	if cap <= 0 {
		panic("agents: NewMemoryQueue cap must be positive")
	}
	return &MemoryQueue{ch: make(chan domain.Task, cap)}
}

// Recv exposes the receive side for a single consumer (or fan out in your own goroutines).
func (q *MemoryQueue) Recv() <-chan domain.Task {
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.Recv")
	if q == nil {
		return nil
	}
	return q.ch
}

// NotifyUserTaskCreated implements [UserTaskCreatedNotifier]. It never blocks: if the buffer is
// full it returns [ErrQueueFull].
func (q *MemoryQueue) NotifyUserTaskCreated(ctx context.Context, task domain.Task) error {
	_ = ctx
	slog.Debug("trace", "cmd", agentsLogCmd, "operation", "agents.MemoryQueue.NotifyUserTaskCreated", "task_id", task.ID)
	if q == nil {
		return nil
	}
	select {
	case q.ch <- task:
		return nil
	default:
		return ErrQueueFull
	}
}
