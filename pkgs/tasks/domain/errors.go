package domain

import "errors"

var (
	// ErrNotFound is returned by store methods when a requested row or
	// resource does not exist. Handlers map this to HTTP 404.
	ErrNotFound = errors.New("tasks: not found")

	// ErrInvalidInput is returned when input fails validation or an
	// operation would violate domain rules (including illegal state
	// transitions surfaced as input errors). Handlers map this to HTTP 400.
	ErrInvalidInput = errors.New("tasks: invalid input")

	// ErrConflict is returned when the request conflicts with current
	// persisted state (for example a uniqueness or ordering constraint).
	// Handlers map this to HTTP 409.
	ErrConflict = errors.New("tasks: conflict")
)
