// Package agents holds automation-side hooks for taskapi, separate from the HTTP handler package.
//
// # User task queue (tradeoffs)
//
// When a human creates a task (POST /tasks with default or X-Actor: user), taskapi can push a snapshot
// of the new row into an optional in-process queue for agent workers bundled in the same process.
//
// In-memory queue (MemoryQueue): zero extra infrastructure, fast, easy to test. Items are lost on
// process crash or restart, and the queue is not shared across multiple taskapi replicas. Backpressure
// is explicit: when the buffer is full, NotifyUserTaskCreated returns ErrQueueFull and the handler
// logs a warning without failing the HTTP request (the task is already persisted).
//
// Alternatives deferred here: a Postgres outbox or external broker (Redis, NATS) for durability and
// multi-instance fan-out; webhook delivery; or consuming only GET /events (SSE carries task ids, not
// full task rows, and is not ideal for offline workers).
//
// Reconciliation: after restarts or if the queue dropped work, ReconcileReadyUserTasksNotQueued
// compares Postgres (ready tasks whose first audit event is user task_created) against the queue's
// pending set and enqueues missing rows. taskapi runs this once at startup when the queue is enabled,
// and optionally on a ticker (T2A_USER_TASK_AGENT_RECONCILE_INTERVAL). Consumers must call AckAfterRecv
// after reading from Recv, or use Receive, so pending ids match the real buffer.
package agents
