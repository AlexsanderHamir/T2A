package agents

import (
	"context"
	"errors"

	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// ErrQueueFull is returned when the in-memory queue cannot accept another task without blocking.
var ErrQueueFull = errors.New("agents: task agent queue full")

// ErrAlreadyQueued is returned when the task id is already tracked as present in the queue buffer.
var ErrAlreadyQueued = errors.New("agents: task already queued")

// UserTaskCreatedNotifier is a legacy hook name; new code should use pkgs/tasks/store.ReadyTaskNotifier
// and (*store.Store).SetReadyTaskNotifier instead.
type UserTaskCreatedNotifier interface {
	NotifyUserTaskCreated(ctx context.Context, task domain.Task) error
}
