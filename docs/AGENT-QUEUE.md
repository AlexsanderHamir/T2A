# Ready-task queue and reconcile (`pkgs/agents`)

How `taskapi` delivers **ready** task snapshots to in-process consumers: store notifier, bounded **`MemoryQueue`**, and periodic reconcile. **Environment variables** (`T2A_USER_TASK_AGENT_*`) are listed in [RUNTIME-ENV.md](./RUNTIME-ENV.md). Architecture context: [DESIGN.md](./DESIGN.md).

## Behavior

`taskapi` **always** wires **`pkgs/agents`** to **`(*store.Store).SetReadyTaskNotifier`**: after the store commits a task with **`status = ready`** (including default-ready creates from any actor) or when a row **transitions** to ready (including dev row mirror), the notifier enqueues a **`domain.Task`** snapshot into a bounded in-memory FIFO for in-process consumers. Mutations still succeed if the queue is full (**`Warn`** log on notify failure). The default buffer depth is **256** (override with **`T2A_USER_TASK_AGENT_QUEUE_CAP`**). This queue is **not** durable and **not** shared across replicas—see package **`pkgs/agents`** (`go doc`) for tradeoffs and future alternatives (outbox, broker).

A background **`agents.RunReconcileLoop`** runs **`ReconcileReadyTasksNotQueued`** once at startup and on a ticker (**default `5m`**, override with **`T2A_USER_TASK_AGENT_RECONCILE_INTERVAL`**; set to **`0`** for startup-only reconcile with no periodic ticker) so **ready** tasks in Postgres that are **not** already tracked as pending in the queue get enqueued again after restarts or drops. Reconcile pages through **`store.ListReadyTaskQueueCandidates`** (bounded pages, not whole-table load) in **oldest `task_created` first** order so backlog is not starved by newer ready tasks or arbitrary UUID ordering; SQLite uses the joined event **`rowid`** as a tie-breaker when timestamps collide. Consumers must **`AckAfterRecv`** (or **`Receive`**) so pending ids match channel contents.

## Related

- [PERSISTENCE.md](./PERSISTENCE.md) — store and audit.
- [API-HTTP.md](./API-HTTP.md) — task REST API (status `ready`).
