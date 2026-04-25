package agents

import "errors"

// ErrQueueFull is returned when the in-memory queue cannot accept another task without blocking.
var ErrQueueFull = errors.New("agents: task agent queue full")

// ErrAlreadyQueued is returned when the task id is already tracked as present in the queue buffer.
var ErrAlreadyQueued = errors.New("agents: task already queued")
