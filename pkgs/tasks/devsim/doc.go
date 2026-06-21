// Package devsim implements optional development-time simulation: synthetic audit events
// and SSE notifications so the UI can be exercised without manual writes.
//
// Enable with environment variable HAMIX_SSE_TEST=1 (see cmd/taskapi). Not for production.
// Optional tuning: HAMIX_SSE_TEST_SYNC_ROW, HAMIX_SSE_TEST_EVENTS_PER_TICK, HAMIX_SSE_TEST_USER_RESPONSE,
// HAMIX_SSE_TEST_LIFECYCLE, HAMIX_SSE_TEST_LIFECYCLE_EVERY (see docs/api.md).
// The package depends on pkgs/tasks/store and pkgs/tasks/domain; callers map ChangeKind to SSE payloads.
package devsim
