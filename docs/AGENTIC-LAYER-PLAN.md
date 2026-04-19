# Agentic layer plan (Cursor CLI)

Long-term plan for evolving the in-process Cursor CLI agent worker into a reliable, multi-runner, multi-replica execution runtime.

V0 (the ready-task queue + reconcile loop) and V1 (the in-process Cursor CLI worker that drives one execution cycle per task through the typed `task_cycles` / `task_cycle_phases` substrate) have shipped. Their contracts live in [AGENT-QUEUE.md](./AGENT-QUEUE.md), [AGENT-WORKER.md](./AGENT-WORKER.md), and [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md). Forward design work for new capabilities goes under [`proposals/`](./proposals/); this doc only tracks the open versions below.

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
- [ ] Add deeper security guardrails (cwd policy beyond V1's `app_settings.repo_root`, additional secret-redaction rules as new runners land).

## V3 — operator-grade observability

Goal: make agent behavior easy to monitor and debug.

Scope:
- Metrics for run duration, success/failure, retries, and task age (V1 ships `t2a_agent_runs_total` + `t2a_agent_run_duration_seconds`; retries / task-age remain).
- Runbook coverage for queue stalls and repeated failures.

TODOs:
- [ ] Add retry/failure counters once V2 retries land.
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

- V2: retries are controlled; duplicate/concurrent execution is prevented.
- V3: operators can detect and triage issues from dashboards + alerts + runbooks.
- V4: system tolerates restarts/replica scaling without losing or starving work.
