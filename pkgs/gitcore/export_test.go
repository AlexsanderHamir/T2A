package gitcore

import "errors"

// NewExecErrorForTest builds an ExecError for unit tests in gitcore_test.
func NewExecErrorForTest(stderr string) error {
	return &ExecError{err: errors.New("exit status 1"), stderr: stderr}
}
