// Package devsim implements optional development-time simulation: synthetic audit events
// and SSE notifications so the UI can be exercised without manual writes.
//
// Enable with environment variable T2A_SSE_TEST=1 (see cmd/taskapi). Not for production.
// The package depends only on pkgs/tasks/store and pkgs/tasks/domain; callers wire SSE
// or other side effects via callbacks.
package devsim
