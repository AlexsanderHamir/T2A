# Agentic layer plan (Cursor CLI)

Simple long-term plan to evolve from today's ready-task queue into a reliable agent worker runtime powered by Cursor CLI.

**Substrate work:** [`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md) is the design + contract for the `task_cycles` / `task_cycle_phases` substrate the V1 worker writes into; [`EXECUTION-CYCLES-PLAN.md`](./EXECUTION-CYCLES-PLAN.md) tracks the staged rollout of that substrate. Backend stages 1–6 are done (domain → store → dual-write → handler → SSE → docs); the web data layer (Stage 7) and UI surface (Stage 8) follow before V1 worker code lands so the worker has both a typed write target and a way for operators to see what it did.

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

Scope:
- One worker loop reads queue task IDs/snapshots.
- Worker reloads latest task from DB before doing work.
- Worker runs Cursor CLI in headless mode (`--print --output-format json`).
- Worker drives one execution cycle per delivered task through the substrate in [`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md): `POST /tasks/{id}/cycles` → walk phases → `PATCH /tasks/{id}/cycles/{cycleId}`. The store's "at most one running cycle per task" guard becomes the worker's per-task claim.
- Worker writes task events/status updates, then acks queue item.

TODOs:
- [x] Provide a typed substrate for worker writes — `task_cycles` / `task_cycle_phases` are live ([`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md)).
- [ ] Define worker state machine (`queued -> running -> done|failed`); map to `task_cycles.status` transitions rather than reinventing state.
- [ ] Add per-run timeout and cancellation.
- [ ] Persist raw CLI result payload (redacted) for debugging — store under `task_cycle_phases.details_json` until artifact volume justifies a dedicated `task_cycle_artifacts` table.
- [ ] Add idempotency key per run (`task_id + attempt + prompt_hash`); use it on `POST /tasks/{id}/cycles` so retries collapse to one cycle row.

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
