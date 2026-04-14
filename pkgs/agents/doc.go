// Package agents holds automation-side hooks for taskapi, separate from the HTTP handler package.
//
// # User task queue (tradeoffs)
//
// When a task becomes ready in the store (create with ready status or status transition to ready), taskapi
// can push a snapshot into an optional in-process queue for agent workers bundled in the same process.
//
// In-memory queue (MemoryQueue): zero extra infrastructure, fast, easy to test. Items are lost on
// process crash or restart, and the queue is not shared across multiple taskapi replicas. Backpressure
// is explicit: when the buffer is full, NotifyReadyTask returns ErrQueueFull and the store
// logs a warning without failing the mutating request (the task row is already persisted).
//
// Alternatives deferred here: a Postgres outbox or external broker (Redis, NATS) for durability and
// multi-instance fan-out; webhook delivery; or consuming only GET /events (SSE carries task ids, not
// full task rows, and is not ideal for offline workers).
//
// Reconciliation: after restarts or if the queue dropped work, ReconcileReadyTasksNotQueued
// compares Postgres (all ready tasks) against the queue's
// pending set and enqueues missing rows. taskapi runs this once at startup and on a ticker by default
// (see T2A_USER_TASK_AGENT_RECONCILE_INTERVAL in docs/RUNTIME-ENV.md and docs/AGENT-QUEUE.md). Consumers must call AckAfterRecv
// after reading from Recv, or use Receive, so pending ids match the real buffer.
package agents
