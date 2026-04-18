# Ready-task queue and reconcile (`pkgs/agents`)

How `taskapi` delivers **ready** task snapshots to in-process consumers: store notifier, bounded **`MemoryQueue`**, and periodic reconcile. **Environment variables** (`T2A_USER_TASK_AGENT_*`) are listed in [RUNTIME-ENV.md](./RUNTIME-ENV.md). Architecture context: [DESIGN.md](./DESIGN.md).

## Behavior

`taskapi` **always** wires **`pkgs/agents`** to **`(*store.Store).SetReadyTaskNotifier`**: after the store commits a task with **`status = ready`** (including default-ready creates from any actor) or when a row **transitions** to ready (including dev row mirror), the notifier enqueues a **`domain.Task`** snapshot into a bounded in-memory FIFO for in-process consumers. Mutations still succeed if the queue is full (**`Warn`** log on notify failure). The default buffer depth is **256** (override with **`T2A_USER_TASK_AGENT_QUEUE_CAP`**). This queue is **not** durable and **not** shared across replicas—see package **`pkgs/agents`** (`go doc`) for tradeoffs and future alternatives (outbox, broker).

A background **`agents.RunReconcileLoop`** runs **`ReconcileReadyTasksNotQueued`** once at startup and on a ticker (**default `5m`**, override with **`T2A_USER_TASK_AGENT_RECONCILE_INTERVAL`**; set to **`0`** for startup-only reconcile with no periodic ticker) so **ready** tasks in Postgres that are **not** already tracked as pending in the queue get enqueued again after restarts or drops. Reconcile pages through **`store.ListReadyTaskQueueCandidates`** (bounded pages, not whole-table load) in **oldest `task_created` first** order so backlog is not starved by newer ready tasks or arbitrary UUID ordering; SQLite uses the joined event **`rowid`** as a tie-breaker when timestamps collide. Consumers must **`AckAfterRecv`** (or **`Receive`**) so pending ids match channel contents.

## Workers and execution cycles

Today's queue only delivers **ready-task snapshots**; it does not record what an in-process consumer does next. The V1 in-process worker (see [AGENT-WORKER.md](./AGENT-WORKER.md) for the contract and [AGENT-WORKER-PLAN.md](./AGENT-WORKER-PLAN.md) for the staged rollout) is the first real consumer: each delivered task id drives one **execution cycle** through the typed `task_cycles` / `task_cycle_phases` substrate documented in [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md). The worker writes through the store directly (not via HTTP) using `StartCycle`, then `StartPhase` / `CompletePhase` for the `diagnose → execute → verify → persist` graph (V1 records `skipped diagnose` + `execute` only), and finally `TerminateCycle`. The store's "at most one running cycle per task" guard doubles as a per-task claim, complementing the queue's dedupe semantics; `AckAfterRecv` runs only **after** terminate so a notify+reconcile race during a long attempt cannot produce a second cycle. The queue itself stays unaware of cycles — it only schedules work; cycles record what the worker did with that work. External clients still drive cycles through the REST routes documented in [API-HTTP.md](./API-HTTP.md) and [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md).

The V1 worker is **opt-in** via `T2A_AGENT_WORKER_ENABLED` (default `false`); when disabled, the queue + reconcile loop run unchanged but no in-process consumer dequeues. See [AGENT-WORKER.md](./AGENT-WORKER.md) for the full env table, security model, and audit shape.

## Related

- [PERSISTENCE.md](./PERSISTENCE.md) — store and audit.
- [API-HTTP.md](./API-HTTP.md) — task REST API (status `ready`).
- [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) — execution-cycle substrate that workers drive after dequeue.
- [AGENT-WORKER.md](./AGENT-WORKER.md) — V1 in-process Cursor CLI consumer of this queue (lifecycle, env vars, security, audit).
- [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) — Cursor CLI worker rollout (V0–V4).
