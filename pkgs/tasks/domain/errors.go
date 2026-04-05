package domain

import "errors"

var (
	ErrNotFound     = errors.New("tasks: not found")
	ErrInvalidInput = errors.New("tasks: invalid input")
	ErrConflict     = errors.New("tasks: conflict")
)
