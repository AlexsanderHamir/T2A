package domain

import (
	"fmt"
	"strings"
)

// RetryMode selects operator retry behavior after a terminal task failure.
type RetryMode string

const (
	RetryFresh  RetryMode = "fresh"
	RetryResume RetryMode = "resume"
)

// PendingRetry is ephemeral intent set by POST /tasks/{id}/retry and consumed
// when the worker transitions the task from ready to running.
type PendingRetry struct {
	Mode          RetryMode `json:"mode"`
	ParentCycleID string    `json:"parent_cycle_id"`
}

// Validate normalizes and checks a pending retry payload.
func (p *PendingRetry) Validate() error {
	if p == nil {
		return fmt.Errorf("%w: pending retry", ErrInvalidInput)
	}
	switch p.Mode {
	case RetryFresh, RetryResume:
	default:
		return fmt.Errorf("%w: retry mode", ErrInvalidInput)
	}
	parent := strings.TrimSpace(p.ParentCycleID)
	if parent == "" {
		return fmt.Errorf("%w: parent_cycle_id", ErrInvalidInput)
	}
	p.ParentCycleID = parent
	return nil
}

// Clone returns a shallow copy for consumption after the row is cleared.
func (p *PendingRetry) Clone() *PendingRetry {
	if p == nil {
		return nil
	}
	cp := *p
	return &cp
}

// Equal reports whether two pending retry payloads match for idempotency.
func (p *PendingRetry) Equal(other *PendingRetry) bool {
	if p == nil || other == nil {
		return p == nil && other == nil
	}
	return p.Mode == other.Mode && p.ParentCycleID == other.ParentCycleID
}
