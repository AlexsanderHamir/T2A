# Ready-task queue and reconcile (`pkgs/agents`)

How `taskapi` delivers **ready** task snapshots to in-process consumers: store notifier, bounded **`MemoryQueue`**, **pickup wake** for deferred `pickup_not_before`, and periodic reconcile. Queue env vars (`T2A_USER_TASK_AGENT_QUEUE_CAP` only) are listed in [RUNTIME-ENV.md](./RUNTIME-ENV.md). Architecture context: [DESIGN.md](./DESIGN.md).

## Behavior

`taskapi` **always** wires **`pkgs/agents`** to **`(*store.Store).SetReadyTaskNotifier`**: after the store commits a task with **`status = ready`** (including default-ready creates from any actor) or when a row **transitions** to ready (including dev row mirror), the notifier enqueues a **`domain.Task`** snapshot into a bounded in-memory FIFO for in-process consumers. Mutations still succeed if the queue is full (**`Warn`** log on notify failure). The default buffer depth is **256** (override with **`T2A_USER_TASK_AGENT_QUEUE_CAP`**). This queue is **not** durable and **not** shared across replicas—see package **`pkgs/agents`** (`go doc`) for tradeoffs and future alternatives (outbox, broker).

**Pickup wake:** `taskapi` also registers **`(*store.Store).SetPickupWake`** with **`agents.PickupWakeScheduler`**. When a ready row has **`pickup_not_before`** in the future, the store schedules a wake at that instant instead of pushing onto the queue immediately. The scheduler uses a min-heap and one timer; at startup **`Hydrate`** loads deferred rows via **`ListDeferredReadyPickupTasks`**. Shutdown calls **`Stop`** on the scheduler before the reconcile goroutine is cancelled. See [SCHEDULING.md](./SCHEDULING.md).

A background **`agents.RunReconcileLoop`** runs **`ReconcileReadyTasksNotQueued`** once at startup and on a fixed ticker (**`agents.ReconcileTickInterval`**, 2 minutes; not configurable via env) so **ready** tasks in Postgres that are **not** already tracked as pending in the queue get enqueued again after restarts or drops. Reconcile pages through **`store.ListReadyTaskQueueCandidates`** (bounded pages, not whole-table load) in **oldest `task_created` first** order so backlog is not starved by newer ready tasks or arbitrary UUID ordering; SQLite uses the joined event **`rowid`** as a tie-breaker when timestamps collide. Consumers must **`AckAfterRecv`** (or **`Receive`**) so pending ids match channel contents.

## Workers and execution cycles

Today's queue only delivers **ready-task snapshots**; it does not record what an in-process consumer does next. The V1 in-process worker (see [AGENT-WORKER.md](./AGENT-WORKER.md) for the contract) is the first real consumer: each delivered task id drives one **execution cycle** through the typed `task_cycles` / `task_cycle_phases` substrate documented in [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md). The worker writes through the store directly (not via HTTP) using `StartCycle`, then `StartPhase` / `CompletePhase` for the `diagnose → execute → verify → persist` graph (V1 records `skipped diagnose` + `execute` only), and finally `TerminateCycle`. The store's "at most one running cycle per task" guard doubles as a per-task claim, complementing the queue's dedupe semantics; `AckAfterRecv` runs only **after** terminate so a notify+reconcile race during a long attempt cannot produce a second cycle. The queue itself stays unaware of cycles — it only schedules work; cycles record what the worker did with that work. External clients still drive cycles through the REST routes documented in [API-HTTP.md](./API-HTTP.md) and [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md).

The V1 worker is **enabled by default** as soon as a workspace repo is configured; it can be toggled live from the SPA Settings page (`app_settings.worker_enabled`). When disabled — or when `app_settings.repo_root` is empty — the queue + reconcile loop run unchanged but no in-process consumer dequeues. See [SETTINGS.md](./SETTINGS.md) and [AGENT-WORKER.md](./AGENT-WORKER.md) for the full configuration surface, security model, and audit shape.

## Related

- [PERSISTENCE.md](./PERSISTENCE.md) — store and audit.
- [API-HTTP.md](./API-HTTP.md) — task REST API (status `ready`).
- [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) — execution-cycle substrate that workers drive after dequeue.
- [AGENT-WORKER.md](./AGENT-WORKER.md) — V1 in-process Cursor CLI consumer of this queue (lifecycle, env vars, security, audit).
- [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) — Cursor CLI worker rollout (V0–V4).
