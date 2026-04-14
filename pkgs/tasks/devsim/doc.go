// Package devsim implements optional development-time simulation: synthetic audit events
// and SSE notifications so the UI can be exercised without manual writes.
//
// Enable with environment variable T2A_SSE_TEST=1 (see cmd/taskapi). Not for production.
// Optional tuning: T2A_SSE_TEST_SYNC_ROW, T2A_SSE_TEST_EVENTS_PER_TICK, T2A_SSE_TEST_USER_RESPONSE,
// T2A_SSE_TEST_LIFECYCLE, T2A_SSE_TEST_LIFECYCLE_EVERY (see docs/API-SSE.md).
// The package depends on pkgs/tasks/store and pkgs/tasks/domain; callers map ChangeKind to SSE payloads.
package devsim
