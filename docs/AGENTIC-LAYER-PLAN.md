# Agentic layer plan (Cursor CLI)

Simple long-term plan to evolve from today's ready-task queue into a reliable agent worker runtime powered by Cursor CLI.

**Substrate work:** [`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md) is the design + contract for the `task_cycles` / `task_cycle_phases` substrate the V1 worker writes into; [`EXECUTION-CYCLES-PLAN.md`](./EXECUTION-CYCLES-PLAN.md) tracks the staged rollout. **The substrate slice is complete** (stages 1–9): domain types, schema + CRUD, dual-write mirror into `task_events`, six new HTTP routes, `task_cycle_changed` SSE, contract docs, web data layer with granular cycle-cache invalidation, and a final integration sweep. The optional UI panel (stage 8) was deliberately deferred — mirror `task_events` already render via `TaskUpdatesTimeline`, so cycle activity is visible without a dedicated panel; promote it when a worker actually drives cycles in production. The worker can now land against a stable typed write target, and any UI a user needs to monitor it is one cached query away (`useTaskCycles` / `useTaskCycle`).

## V0 (now) — queue foundation

What we already have:
- Ready-task notifier from store commits.
- Bounded in-memory `MemoryQueue` with dedupe (`ErrAlreadyQueued`) and backpressure (`ErrQueueFull`).
- Startup + periodic reconcile (`ReconcileReadyTasksNotQueued`).
- Basic queue metrics and logs.

TODOs:
- [ ] Keep `docs/AGENT-QUEUE.md` and `docs/RUNTIME-ENV.md` aligned when queue behavior changes.
- [ ] Add dashboard panels for queue fill ratio, reconcile enqueue counts, and queue-full events.

## V1 — single worker with Cursor CLI

Goal: process ready tasks end-to-end in one process reliably enough for early production.

Scope (one paragraph; the per-stage rollout, edge cases, and exit criteria live in [`AGENT-WORKER-PLAN.md`](./AGENT-WORKER-PLAN.md) — track V1 status there, not here):

- One in-process worker loop consumes the existing `MemoryQueue`, reloads the task, runs Cursor CLI in headless mode (`--print --output-format json`) behind a small `runner.Runner` interface so Claude / Codex / future CLIs land as additional adapters, and writes the attempt through the [`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md) substrate (one `task_cycle` + one `execute` phase in V1; per-phase decomposition is V2). The store's "at most one running cycle per task" guard becomes the worker's per-task claim. A startup sweep clears orphan `running` cycles from previous processes so a hard kill cannot wedge a task.

## V2 — reliability and safety

Goal: make failures predictable and recoverable.

Scope:
- Retry/backoff policy with attempt caps.
- Validation layer for CLI output before applying updates.
- Better failure taxonomy (transient vs terminal).

TODOs:
- [ ] Add retry policy and terminal-failure rule.
- [ ] Add task-level lock/claim to prevent concurrent processing of same task.
- [ ] Add prompt/version tracking for reproducibility.
- [ ] Add security guardrails (env allowlist, cwd policy, secret redaction).

## V3 — operator-grade observability

Goal: make agent behavior easy to monitor and debug.

Scope:
- Metrics for run duration, success/failure, retries, and task age.
- Runbook coverage for queue stalls and repeated failures.

TODOs:
- [ ] Add metrics: `agent_runs_total`, `agent_run_duration_seconds`, retry/failure counters.
- [ ] Add alert rules for sustained queue-full and high terminal-failure rate.
- [ ] Add correlation IDs across queue event -> worker run -> task_events writes.
- [ ] Add runbooks for "stuck queue" and "high retry burn."

## V4 — durability and scale

Goal: support multi-instance processing with stronger guarantees.

Scope options (choose one when needed):
- Database lease/claim worker model, or
- External durable broker (Redis/NATS/SQS style).

TODOs:
- [ ] Decide delivery model target (at-least-once vs effectively-once with idempotency).
- [ ] Implement visibility timeout / reclaim flow for crashed workers.
- [ ] Add dead-letter handling for repeatedly failing tasks.
- [ ] Load-test queue/worker behavior at target concurrency.

## Exit criteria by version

- V1: tasks can be processed end-to-end with bounded timeout and durable result writes.
- V2: retries are controlled; duplicate/concurrent execution is prevented.
- V3: operators can detect and triage issues from dashboards + alerts + runbooks.
- V4: system tolerates restarts/replica scaling without losing or starving work.
