package agents

import (
	"context"
	"errors"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ErrQueueFull is returned when the in-memory queue cannot accept another task without blocking.
var ErrQueueFull = errors.New("agents: user task queue full")

// ErrAlreadyQueued is returned when the task id is already tracked as present in the queue buffer.
var ErrAlreadyQueued = errors.New("agents: task already queued")

// UserTaskCreatedNotifier is invoked by the tasks HTTP handler after a successful user-originated
// POST /tasks (persisted task and tree load). Implementations must be non-blocking for the HTTP
// caller unless they complete quickly (for example a buffered channel send).
type UserTaskCreatedNotifier interface {
	NotifyUserTaskCreated(ctx context.Context, task domain.Task) error
}
