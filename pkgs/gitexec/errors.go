package gitexec

import "errors"

// ErrNotFound is returned when a git object or revision does not exist in the
// requested repository. HTTP handlers map this to 404.
var ErrNotFound = errors.New("not found")
