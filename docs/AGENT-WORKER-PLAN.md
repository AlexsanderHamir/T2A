# Agent worker (V1) — implementation plan

> **Where this fits.** Long-term roadmap across V0–V4 lives in
> [`AGENTIC-LAYER-PLAN.md`](./AGENTIC-LAYER-PLAN.md). This document is the
> **per-stage execution playbook for V1 only** — commit + push + STOP gates,
> edge cases, and exit criteria. It will be archived once V1 ships. Same
> split pattern as [`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md) (long-term
> contract) vs [`EXECUTION-CYCLES-PLAN.md`](./EXECUTION-CYCLES-PLAN.md)
> (stage rollout).

This document is the **agreed working breakdown** for putting the first real
consumer behind the ready-task queue: an in-process worker that drives a task
end-to-end through the **diagnose → execute → verify → persist** loop defined
in [`moat.md`](../moat.md), records each attempt as one `task_cycle` + one
`execute` phase row (already mirrored into `task_events` by the store), and
acks the queue.

Until this lands, T2A has every piece of the moat in plumbing but never
actually runs the loop. This plan is the smallest slice that fixes that.

**Design rationale and tradeoffs:** kept inline in this doc; promote to a
contract doc (`docs/AGENT-WORKER.md`) in **Stage 5**.

## Rules of engagement

1. **One stage per PR / commit.** Stages are sized so each leaves the repo
   **buildable, tested, linted, and shippable**.
2. **Verification gate per stage.** A stage is not "done" until its checklist
   is green AND `./scripts/check.ps1` (or the documented Go-only fast path)
   passes locally.
3. **Commit + push at end of stage.** Conventional commit message, one
   logical concern, push to current branch (`main` unless redirected). See
   [`AGENTS.md`](../AGENTS.md) "Commands to run before you finish".
4. **STOP and ask permission between stages.** No silent rollover; the user
   explicitly OKs each next stage.
5. **TDD default** per [`AGENTS.md`](../AGENTS.md): failing test first when
   adding behavior, then make it green.
6. **No scope creep.** If a stage discovers extra work, append it to the
   `### Notes / followups` block at the bottom of this file rather than
   expanding the active stage.

## Reference points

- **Substrate already in code:** `pkgs/tasks/store/store_cycles.go` and
  `store_cycle_phases.go` (commit `f72ad84`) already implement
  `StartCycle` / `TerminateCycle` / `StartPhase` / `CompletePhase` *with*
  the in-transaction `task_events` mirror. The worker is therefore the
  **first real consumer** of that substrate; this plan does not need to
  wait on Stages 4–9 of [`EXECUTION-CYCLES-PLAN.md`](./EXECUTION-CYCLES-PLAN.md).
- **Closest analog in repo:** the **reconcile loop**
  (`pkgs/agents/reconcile.go::RunReconcileLoop`) — a long-running goroutine
  owned by `cmd/taskapi`, started from `run_helpers.go::startReadyTaskAgents`,
  context-cancelled on shutdown, env-tunable through
  `internal/taskapiconfig`. The worker mirrors this lifecycle.
- **Queue contract the worker consumes:**
  `pkgs/agents/memory_queue.go::(*MemoryQueue).Receive` — blocks until a
  `domain.Task` snapshot arrives, removes the id from the pending set, and
  returns. No external broker, no durability beyond the store.
- **Engineering bar:** `.cursor/rules/BACKEND_AUTOMATION/backend-engineering-bar.mdc`,
  `.cursor/rules/BACKEND_AUTOMATION/observability-logging.mdc`,
  `.cursor/rules/BACKEND_AUTOMATION/go-testing-recipes.mdc`.

## Why a runner abstraction from day one

V1 ships with **Cursor CLI** as the only real runner, but the worker must
not call Cursor directly. Three concrete CLI runtimes are already on the
horizon — Cursor, Claude Code, Codex — and several more will follow. Paying
the abstraction cost **once**, while there is only one implementation to
align it with, avoids the standard failure mode where three runners later
disagree about prompt shape, output JSON, working directory, env handling,
and timeout semantics, and the worker grows runner-specific branches.

Concretely, the seam is:

```
worker ─► AgentRunner interface
            ├── cursor   (V1 — only real adapter)
            ├── claude   (later — new file, no rewrite)
            ├── codex    (later — new file, no rewrite)
            ├── …        (each adapter ≈ one file)
            └── fake     (tests — always shipped)
```

- `AgentRunner` is a **narrow** interface (`Run(ctx, Request) (Result, error)`,
  `Name() string`, `Version() string`) per the engineering bar's
  "accept interfaces, return concrete types" rule. `Version()` is the
  runner-reported identity string the worker writes into
  `task_cycles.MetaJSON.runner_version` for every attempt (see Stage 3
  step 3); for Cursor that's the `cursor --version` string captured
  at Stage 4's startup probe and cached on the adapter.
- The `task_cycles.MetaJSON` column already exists; the worker writes
  `{"runner":"cursor-cli","runner_version":"…","prompt_hash":"…"}` into it
  on every cycle so the audit trail is honest about who ran each attempt.
  No schema change required to support multi-runner.
- **Runner selection is out of scope for V1**: `cmd/taskapi` hardcodes the
  Cursor adapter. A `T2A_AGENT_RUNNER` env knob can be added in V2 when a
  second adapter exists.

## Edge cases V1 must handle

These are the edge cases this plan **explicitly addresses** because each
one would make V1 ship broken or silently degrade in production. Each is
implemented in the stage noted in parentheses; the design rationale lives
inline so we do not re-litigate later.

1. **Two phase rows per cycle, only one of substance.** Cursor CLI is
   a single headless invocation; there is no honest way to slice it
   into the four `moat.md` phases without inventing four prompts we
   have not designed yet. The substrate's `domain.ValidPhaseTransition`
   (see [`EXECUTION-CYCLES.md`](./EXECUTION-CYCLES.md), state machine
   section) requires the **first phase on a cycle to be `diagnose`** —
   `StartPhase(execute)` on a fresh cycle is rejected with
   `phase transition "" -> "execute" not allowed`. V1 therefore writes
   two rows: a `diagnose` row immediately closed with `status="skipped"`
   and `summary="single-phase V1; diagnose deferred"` (mirrored as
   `phase_skipped` in `task_events` so the audit trail is honest), then
   an `execute` row that wraps the actual Cursor invocation. The
   `diagnose → execute → verify → persist` decomposition is how the
   substrate is **shaped**; it is not how the V1 worker uses it. Real
   per-phase decomposition is deferred to V2 once we either (a) wrap
   Cursor with distinct shell pre/post steps or (b) add a runner whose
   CLI exposes per-phase calls. (Stage 3.)
2. **Worker transitions task status.** Without this, the reconcile loop
   re-enqueues the same `ready` task on every 5m tick forever. Worker
   flips `ready → running` at `StartCycle` and `running → done | failed`
   at `TerminateCycle`, in the same store call where possible. (Stage 3.)
3. **Orphan `running` cycles after crash / restart.** `StartCycle`
   refuses to begin a new attempt while a `running` cycle exists, which
   means a single hard-killed process can permanently wedge a task. A
   **startup sweep** runs once before the worker loop starts and aborts
   any `task_cycles` rows still in `running` from a previous process,
   marking them `aborted` with reason `process_restart` and flipping
   the underlying task back to `ready`. Same sweep handles orphan running
   phases. (Stage 4.)
4. **Worker survives one bad task.** A panic in the runner adapter
   (JSON parse, nil deref, anything) inside a single goroutine would
   stop **all** further task processing until the next process restart.
   `processOne` wraps every per-task body in `defer recover()`, logs
   the panic with stack, best-effort terminates the cycle as `failed`
   with reason `panic`, acks, and the loop continues. (Stage 3.)
5. **Graceful shutdown does not leak `running` cycles.** When the
   shutdown context fires while `runner.Run` is in flight, the worker
   best-effort kills the child process (already inherent in Stage 2),
   then issues `TerminateCycle(..., aborted, "shutdown")` using a
   **separate** non-cancelled context with a 5s deadline so the audit
   row lands even though the request context is dead. The startup sweep
   from edge case #3 is the safety net if even that fails. (Stages 3 + 4.)
6. **UI stays alive during worker runs.** Cycle/phase writes happen at
   the store layer; SSE `notifyChange` lives in the handler layer. Worker
   takes an **optional** `CycleChangeNotifier` interface (defined in
   `pkgs/agents/worker`) with signature
   `PublishCycleChange(taskID, cycleID string)`, and `cmd/taskapi`
   wires `SSEHub` into it via a tiny adapter that publishes a
   **`task_cycle_changed`** frame (`{type, id=task, cycle_id=cycle}`)
   after each cycle/phase write. The `task_cycle_changed` event type
   shipped in Stage 5 of `EXECUTION-CYCLES-PLAN.md` (commit `0b2be37`)
   and the SPA already routes those frames to the dedicated cycles
   cache slot (`["tasks","detail",id,"cycles"]`) via
   `parseTaskChangeFrame` / `useTaskEventStream` (Stage 7,
   commit `d5948d2`), so checklist / events / detail caches stay warm
   on open task pages and only the cycles list / cycle detail
   refetches. The V1 worker becomes the first server-side publisher of
   the existing event type — no new SSE event type, no
   handler-package change. (Stages 3 + 4.)

The edge cases below are flagged but **not** addressed in V1; they live
in `### Notes / followups` so they cannot get lost: working-directory
hygiene between sequential runs, `task` cascade-delete mid-cycle,
explicit byte caps on `RawOutput` / `Details`, redelivery after
`AckAfterRecv` but before `TerminateCycle`, prompt-secret redaction in
slog output, and a startup probe of the Cursor binary itself.

## What we're building (one-screen recap)

A single-goroutine in-process consumer of the existing `MemoryQueue`. For
each ready task it receives, it:

1. Reloads the latest task row from the store (the queued snapshot can be
   stale). If status is no longer `ready`, log + ack + return.
2. Transitions the task `ready → running` and calls
   `(*store.Store).StartCycle(...)` with `runner` metadata in `MetaJSON`
   (`{"runner": "cursor-cli", "runner_version": "...", "prompt_hash": "..."}`).
3. **Two phase rows** (edge case #1): a no-op `diagnose` row
   immediately closed with `status="skipped"` to satisfy the
   substrate's "first phase must be `diagnose`" rule, then the real
   `execute` row — `StartPhase(execute)` → `runner.Run(...)` →
   `CompletePhase(succeeded | failed)` with the result in `details`.
4. Calls `TerminateCycle(...)` with `succeeded` or `failed`, and
   transitions the task to `done` or `failed` accordingly.
5. Calls `(*MemoryQueue).AckAfterRecv(taskID)` — **after** step 4, never
   before, so a redelivery in the gap cannot create a duplicate cycle.

V1 runs **one** worker goroutine. No retries, no concurrency, no leases,
no dead-letter, no per-phase decomposition — every one of those belongs
to V2/V3 of [`AGENTIC-LAYER-PLAN.md`](./AGENTIC-LAYER-PLAN.md) and is
intentionally deferred so this slice stays narrow enough to fit in one
PR cycle per stage.

## Stages

Each stage's "Exit criteria" is the gate. Verification commands are listed
once at the bottom under [Common verification](#common-verification).

---

### Stage 0 — Plan landed (this doc)

- [x] `docs/AGENT-WORKER-PLAN.md` written.
- [x] Linked from `docs/AGENTIC-LAYER-PLAN.md` (V1 section, line 23)
  and `docs/README.md` (index row + cross-reference table).
- [x] Drift fixes folded in before Stage 1 begins, against the
  substrate as it actually shipped (Stages 1–9 of
  `EXECUTION-CYCLES-PLAN.md`): edge case #1 acknowledges the
  "first phase must be `diagnose`" rule and writes a skipped diagnose
  row before execute; edge case #6 + Stage 4 SSE adapter publish the
  `task_cycle_changed` event type that shipped in Stage 5
  (commit `0b2be37`) so the SPA's granular cycles-cache invalidation
  (Stage 7, commit `d5948d2`) lights up immediately; deferred-list
  bullets refreshed to reflect that the cycle REST routes (`9151a58`)
  and `task_cycle_changed` SSE event have already shipped; runner
  interface signature gains `Version() string` so Stage 3's
  `MetaJSON.runner_version` write line up with Stage 1's interface.
- [x] Commit + push.

**Commit:** `docs: add agent worker (V1) implementation plan`

**STOP — ask permission to begin Stage 1.**

---

### Stage 1 — Runner interface + fake (no I/O, no DB, no exec)

**Scope (touch only `pkgs/agents/runner/`):**

- [ ] New package `pkgs/agents/runner` with:
  - `runner.go` defining `Runner` interface
    (`Run(ctx context.Context, req Request) (Result, error)`,
    `Name() string`, `Version() string`), `Request`, `Result`, and
    typed errors (`ErrTimeout`, `ErrNonZeroExit`, `ErrInvalidOutput`).
  - `Request` carries: `TaskID`, `AttemptSeq`, `Phase` (one of the four
    `domain.Phase` values), `Prompt`, `WorkingDir`, `Timeout`, `Env`
    (allowlisted map; PATH/HOME passed through, nothing else by default).
  - `Result` carries: `Status` (`domain.PhaseStatus`), `Summary` (≤512
    chars, free text for the phase row), `Details` (`json.RawMessage`),
    `RawOutput` (capped, redacted). All are JSON-serialisable.
- [ ] `pkgs/agents/runner/runnerfake/runnerfake.go` with a deterministic
  `Runner` impl that returns scripted `Result`s per `(TaskID, Phase)`
  tuple, used by every later test in this plan.
- [ ] `pkgs/agents/runner/doc.go` documenting the interface contract,
  the multi-runner roadmap, and the secret-redaction expectation that
  every adapter must meet.
- [ ] Table-driven `runner_test.go` pinning request/result JSON shapes
  (so adapters serialise into the same wire format).

**Exit criteria:**

- `go vet ./pkgs/agents/runner/...` clean.
- `go test ./pkgs/agents/runner/... -count=1` passes.
- `funclogmeasure -enforce` green on touched files.
- No changes outside `pkgs/agents/runner/`.

**Commit:** `agents: add Runner interface, Request/Result types, and fake runner`

**STOP — ask permission to begin Stage 2.**

---

### Stage 2 — Cursor CLI adapter (still no worker loop)

**Scope (touch only `pkgs/agents/runner/cursor/`):**

- [x] New package `pkgs/agents/runner/cursor` with:
  - `cursor.go` implementing the Stage-1 `Runner` interface by shelling
    out to `cursor --print --output-format json` (or the current Cursor
    headless invocation — pin in code comment).
  - Exec is dependency-injected through a private `execFn` field so unit
    tests do not shell out. The default is `exec.CommandContext`.
  - Per-call timeout enforced through `context.WithTimeout`; on timeout
    return `runner.ErrTimeout` and best-effort kill the child process.
  - Env passed through is **opt-in**: only entries from the request's
    `Env` map plus `PATH`, `HOME`, `USERPROFILE` (Windows). `DATABASE_URL`
    and any `T2A_*` are explicitly excluded — runners never see store
    credentials.
  - JSON output parsed into `runner.Result`; non-zero exit codes mapped
    to `runner.ErrNonZeroExit` with the redacted tail of stderr in
    `Result.Details`.
- [x] `cursor_test.go` with a fake `execFn` covering: success path,
  non-zero exit, JSON parse failure, timeout, output redaction (no
  `Authorization`, no `T2A_*`, no absolute home paths in `RawOutput`).
- [x] Adapter is **not** wired into any binary yet.

**Out of scope for this stage:** any worker goroutine, any DB writes,
any change to `cmd/taskapi`. Pure adapter + tests.

**Exit criteria:**

- `go test ./pkgs/agents/runner/... -count=1` passes (fake exec only;
  no real Cursor binary required for `go test`).
- `funclogmeasure -enforce` clean.

**Commit:** `agents: add Cursor CLI adapter for Runner interface`

**STOP — ask permission to begin Stage 3.**

---

### Stage 3 — Worker loop (in-process consumer)

**Scope (touch only `pkgs/agents/worker/`):**

- [x] New package `pkgs/agents/worker` with:
  - `worker.go` exposing `type Worker struct{ … }` and
    `NewWorker(store *store.Store, queue *agents.MemoryQueue, runner runner.Runner, opts Options) *Worker`.
  - `Worker.Run(ctx context.Context) error` — single goroutine; loops
    `queue.Receive` → `processOne` until ctx cancels.
  - `processOne(ctx, task domain.Task)` runs inside a `defer recover()`
    block (edge case #4): a panic logs the stack, attempts a best-effort
    `TerminateCycle(..., failed, "panic")`, then acks and returns so the
    loop continues.
  - `processOne` body:
    1. Reload task by id from store. If status is no longer `ready`,
       log `Warn` ("stale task at dequeue"), ack queue, return.
    2. Transition task `ready → running` (existing store entry point;
       if the transition fails because the row vanished — task was
       deleted — log `Info`, ack, return per edge case below).
    3. `StartCycle` with `MetaJSON = {"runner": runner.Name(),
       "runner_version": runner.Version(),
       "prompt_hash": sha256(task.InitialPrompt)}`.
    4. **Two phase rows, one of substance** (edge case #1): the
       substrate's `ValidPhaseTransition` requires the first phase on
       a cycle to be `diagnose`, so V1 writes a no-op `diagnose` row
       first to satisfy the state machine, then the real `execute`
       row. Concretely:
       - `StartPhase(ctx, cycle.ID, PhaseDiagnose, ActorAgent)` →
         immediately `CompletePhase(... PhaseStatusSkipped, summary="single-phase V1; diagnose deferred", details=nil)`
         (mirrors as `phase_skipped` in `task_events`).
       - `StartPhase(ctx, cycle.ID, PhaseExecute, ActorAgent)` →
         `runner.Run(...)` → `CompletePhase` with status `succeeded`
         for `nil` error, `failed` for `runner.ErrNonZeroExit` /
         `ErrInvalidOutput` / `ErrTimeout` (mirrors as
         `phase_completed` / `phase_failed`).
       The diagnose row's `event_seq` will point at its own
       `phase_skipped` mirror; the execute row's `event_seq` will
       point at its terminal mirror per the dual-write contract.
    5. `TerminateCycle(ctx, cycle.ID, succeeded|failed, reason, ActorAgent)`.
    6. Transition task `running → done` (succeeded) or `running → failed`
       (failed) so the reconcile loop does not re-enqueue (edge case #2).
    7. Log one structured summary line: task id, attempt seq, cycle id,
       terminal status, total duration, runner name + version.
    8. `(*MemoryQueue).AckAfterRecv(taskID)` — **last step**, after
       `TerminateCycle` succeeds, so a redelivery during the run cannot
       race with a duplicate `StartCycle`.
  - **Shutdown handling** (edge case #5): `processOne` watches
    `ctx.Done()`. On cancel mid-`runner.Run`, the runner kill-on-ctx
    behaviour (Stage 2) tears down the child process; the worker then
    runs a final block with **`context.WithTimeout(context.Background(),
    5*time.Second)`** to write `TerminateCycle(..., aborted, "shutdown")`
    and the matching `running → failed` task transition. If even that
    deadline trips, the startup sweep in Stage 4 is the safety net.
  - `CycleChangeNotifier` interface (edge case #6) defined in this
    package: `PublishCycleChange(taskID, cycleID string)`. `Options`
    holds an optional `Notifier CycleChangeNotifier`. Worker calls
    `notifier.PublishCycleChange(...)` after each successful
    `StartCycle`, `StartPhase`, `CompletePhase`, `TerminateCycle`. Nil
    notifier is a no-op so the package stays unit-testable without an
    SSE hub.
  - `Options`: per-run timeout (default 5m), shutdown abort timeout
    (default 5s), optional clock (for tests), optional notifier.
- [x] `worker_test.go` driving the worker with the fake `MemoryQueue`,
  the SQLite test store (`internal/tasktestdb.OpenSQLite`), and the
  `runnerfake.Runner` from Stage 1. Cases:
  - **Happy path:** task → cycle row + 2 phase rows (skipped diagnose
    + succeeded execute) + 6 mirror events (`cycle_started`,
    `phase_started`+`phase_skipped` for diagnose,
    `phase_started`+`phase_completed` for execute, `cycle_completed`)
    appear in store; task ends in `done`.
  - **Runner failure:** runner returns `ErrNonZeroExit` → cycle row
    `failed`, phase row `failed`, `cycle_failed` mirror event present,
    task ends in `failed`, queue is acked.
  - **Stale task at dequeue:** task was completed via REST between
    enqueue and dequeue → no cycle written, queue is acked, single
    `Warn` log.
  - **Task deleted mid-cycle:** delete row after `StartPhase` but before
    runner returns → `CompletePhase` returns `ErrNotFound`, worker logs
    `Info`, acks, no panic.
  - **Panic in runner** (edge case #4): runner panics inside `Run` →
    `defer recover()` catches it, cycle is terminated as `failed` with
    reason `panic`, worker loop continues processing next queued task.
  - **Shutdown mid-run** (edge case #5): cancel parent ctx while runner
    is sleeping → cycle ends in `aborted`, task ends in `failed`,
    worker loop returns nil error.
  - **No double cycle on redelivery:** simulate the queue producing the
    same task id twice (notify + reconcile race); second `StartCycle`
    yields `ErrInvalidInput` ("running cycle exists"), worker logs and
    acks without crashing.
  - **Notifier publishes once per write** (edge case #6): fake notifier
    records calls; happy path produces exactly **6** publishes (one
    per cycle/phase mutation: `StartCycle`, diagnose `StartPhase`,
    diagnose `CompletePhase(skipped)`, execute `StartPhase`, execute
    `CompletePhase`, `TerminateCycle`), each carrying the cycle id;
    nil notifier is a no-op.
  - Race detector: `go test -race ./pkgs/agents/worker/...`.

**Out of scope for this stage:** retries, multiple workers, runner
selection, metrics, Cursor binary probe, startup orphan sweep. Wiring
into `cmd/taskapi` (including the SSE adapter and the orphan sweep) is
Stage 4.

**Exit criteria:**

- `go test ./pkgs/agents/worker/... -count=1 -race` passes.
- `go test ./pkgs/tasks/store/... -count=1` still passes (no store
  signature changes).
- `funclogmeasure -enforce` clean.

**Commit:** `agents: add single-goroutine worker that drives diagnose→execute→verify→persist cycle`

**STOP — ask permission to begin Stage 4.**

---

### Stage 4 — `cmd/taskapi` wiring + config + startup sweep

**Scope (touch `cmd/taskapi/run_helpers.go`, `internal/taskapiconfig/env.go`,
`pkgs/agents/runner/cursor/`, and a new `pkgs/agents/worker/sweep.go`):**

- [x] Extend `internal/taskapiconfig` with:
  - `EnvAgentWorkerEnabled = "T2A_AGENT_WORKER_ENABLED"` (default `false`
    — fail-safe; users opt in once they have Cursor CLI on PATH).
  - `EnvAgentWorkerCursorBin = "T2A_AGENT_WORKER_CURSOR_BIN"` (default
    `cursor`; lets ops point at an absolute path).
  - `EnvAgentWorkerRunTimeout = "T2A_AGENT_WORKER_RUN_TIMEOUT"`
    (default `5m`).
  - `EnvAgentWorkerWorkingDir = "T2A_AGENT_WORKER_WORKING_DIR"`
    (default: `REPO_ROOT` if set, else process cwd; fail-fast at startup
    if directory does not exist).
  - Tests in `env_test.go` mirroring existing patterns (default,
    override, invalid → log + default).
- [x] **Startup sweep** (edge case #3) in `pkgs/agents/worker/sweep.go`:
  - `func SweepOrphanRunningCycles(ctx context.Context, st *store.Store) (SweepResult, error)`
    — finds all `task_cycles` with `status='running'`, marks each
    `aborted` with reason `process_restart` (writes `cycle_failed`
    audit mirror with the reason), and transitions the underlying task
    `running → failed`. Idempotent: re-running it on an already-clean
    DB is a no-op.
  - Same treatment for orphan running phases inside non-running cycles
    (`status='running'` phase under a now-terminal cycle gets flipped
    to `failed` with reason `process_restart`).
  - Test in `sweep_test.go` covering: clean DB no-op, single orphan
    cycle promoted to `aborted` with audit mirror, orphan phase under
    aborted cycle promoted to `failed`, task status correctly walked
    back to `failed`.
  - **Runs once at startup before the worker loop begins** (in
    `cmd/taskapi/run_helpers.go`); logs `Info` with the result.
- [x] **Cursor binary probe** (edge case #7 in followups, addressed
  here cheaply): when `T2A_AGENT_WORKER_ENABLED` is truthy, run
  `cursor --version` with a 5s timeout at startup. On failure, log
  `Error("cursor binary not usable, refusing to start agent worker", …)`
  and **exit 1** — fail-fast per the engineering bar's "fail loudly at
  startup" rule. Skip the probe when the worker is disabled.
- [x] **SSE adapter** (edge case #6): tiny private type in
  `cmd/taskapi/run_helpers.go` that implements
  `worker.CycleChangeNotifier` by calling
  `hub.Publish(handler.TaskChangeEvent{Type: handler.TaskCycleChanged, ID: taskID, CycleID: cycleID})`.
  The `TaskCycleChanged` event type already exists (Stage 5 of
  `EXECUTION-CYCLES-PLAN.md`, commit `0b2be37`) and the SPA already
  routes it to a dedicated cache slot (Stage 7, commit `d5948d2`),
  so no new SSE event type and no handler-package change are needed —
  V1 just becomes the first server-side publisher of the existing
  type.
- [x] `cmd/taskapi/run_helpers.go::startReadyTaskAgents` becomes
  `startReadyTaskAgents(...) (cancel context.CancelFunc, q *agents.MemoryQueue, w *worker.Worker)`:
  - When `T2A_AGENT_WORKER_ENABLED` is truthy:
    1. Probe Cursor binary; exit on failure.
    2. Run `worker.SweepOrphanRunningCycles` once (log result).
    3. Build the Cursor adapter (`runner/cursor.New(...)`) with timeout
       + working dir from config.
    4. Build the SSE notifier adapter that wraps `hub`.
    5. Construct `worker.New(...)` with the notifier in `Options`.
    6. `go w.Run(workerCtx)` in addition to the existing reconcile
       goroutine.
  - When disabled: keep current behaviour (queue + reconcile only,
    **no sweep, no probe**) so operators who do not want Cursor CLI on
    PATH are unaffected.
  - Both goroutines share the `reconcileCancel` lifecycle so shutdown
    drains the worker before closing the DB pool. Shutdown order:
    cancel worker ctx → wait up to `T2A_AGENT_WORKER_RUN_TIMEOUT + 10s`
    for `Worker.Run` to return (to give the in-flight `aborted` write
    from edge case #5 a chance to land) → cancel reconcile ctx →
    proceed to existing DB close.
- [x] One `slog.Info("agent worker config", …)` line at startup with
  `enabled`, `runner`, `cursor_bin` (path only, never args),
  `cursor_version` (from probe), `run_timeout_sec`, `working_dir`.
- [x] No new HTTP routes, no new SSE event types.

**Out of scope for this stage:** Prometheus metrics, alert rules,
runbooks (those are Stage 6 of `AGENTIC-LAYER-PLAN.md` V3 territory).

**Exit criteria:**

- `go vet ./...` clean.
- `go test ./... -count=1` passes (worker stays disabled by default in
  every existing test path; new env-disabled tests pin that fact).
- `funclogmeasure -enforce` clean.

**Commit:** `taskapi: wire optional Cursor CLI agent worker behind T2A_AGENT_WORKER_ENABLED`

**STOP — ask permission to begin Stage 5.**

---

### Stage 5 — Backend docs + contract pinning

**Scope (docs-only, no code changes):**

- [x] New `docs/AGENT-WORKER.md` — lifecycle, runner abstraction, env
  vars, security model (env allowlist, secret redaction), how it
  composes with the queue + cycles substrate, what an operator sees in
  logs / `task_events` for one happy-path attempt, and the explicit
  V2/V3 deferrals.
- [x] `docs/AGENT-QUEUE.md` — short note: "consumers" is no longer
  hypothetical; link to `AGENT-WORKER.md`.
- [x] `docs/AGENTIC-LAYER-PLAN.md` — strike-through / check the V1 line
  items satisfied by this slice; link to `AGENT-WORKER.md`.
- [x] `docs/RUNTIME-ENV.md` — add `T2A_AGENT_WORKER_*` rows to the env
  table.
- [x] `docs/DESIGN.md` — one new row in the contract docs table
  pointing at `AGENT-WORKER.md`; one new "Limitations" bullet noting
  the worker is in-process and does not coordinate across replicas.
- [x] `docs/README.md` index — new row for `AGENT-WORKER.md` and
  `AGENT-WORKER-PLAN.md`.
- [x] `AGENTS.md` repo-map — new rows for `pkgs/agents/runner/`,
  `pkgs/agents/runner/cursor/`, `pkgs/agents/worker/`.

**Exit criteria:**

- `./scripts/check.ps1` with `CHECK_SKIP_WEB=1` (docs-only fast path).
- All cross-links resolve (manual scan).

**Commit:** `docs: document agent worker lifecycle, env vars, and runner abstraction`

**STOP — ask permission to begin Stage 6.**

---

### Stage 6 — Observability + integration sweep

**Scope:**

- [ ] Add a single Prometheus counter
  `t2a_agent_runs_total{runner,terminal_status}` registered through
  `internal/taskapi` next to `RegisterAgentQueueMetrics`. Increment in
  `worker.processOne` after `TerminateCycle`. Cardinality is bounded
  (one runner today, four terminal statuses).
- [ ] Add a histogram `t2a_agent_run_duration_seconds{runner}` with
  the existing histogram bucket convention used in `pkgs/tasks/store/store_metrics.go`.
- [ ] Update `deploy/prometheus/t2a-taskapi-rules.yaml` only if the
  user explicitly wants alerts; otherwise note in Stage 6 commit body
  that alerts are deferred to V3 of `AGENTIC-LAYER-PLAN.md`.
- [ ] One new end-to-end test under `pkgs/tasks/agentreconcile` (or a
  sibling `pkgs/agents/agentworker_e2e_test.go`) that:
  - starts a real SQLite store + queue + reconcile + worker (fake
    runner),
  - inserts a ready task,
  - waits for the cycle to terminate,
  - asserts the full sequence of `task_events` rows and the queue's
    pending-set state at the end.
- [ ] Re-read `docs/AGENT-WORKER.md` for drift introduced by Stages 4–5.
- [ ] Append a final "V1 worker shipped" note in
  `### Notes / followups` below.

**Exit criteria:**

- Full `./scripts/check.ps1` (no skip flags) green.
- `funclogmeasure -enforce` across the whole repo green.
- This file: every checkbox checked, status table updated.

**Commit:** `chore: finalize agent worker V1 (metrics + e2e + docs sweep)`

---

## Common verification

| Before commit (per stage) | Command |
|---|---|
| Go-only stages (1–4, 6) | `go vet ./... ; go test ./... -count=1 ; go run ./cmd/funclogmeasure -enforce` |
| Concurrency-touching stages (3, 6) | also `go test -race ./pkgs/agents/...` |
| Docs-only stage (5) | `$env:CHECK_SKIP_WEB='1' ; .\scripts\check.ps1` |
| Full pass (Stage 6) | `.\scripts\check.ps1` |

`gofmt -w` on touched `*.go` files always.

## What's deliberately deferred (not scope)

- **Per-phase decomposition (`diagnose → execute → verify → persist`
  inside a single cycle).** V1 records exactly one `execute` phase per
  cycle (edge case #1). Earning the four-phase split needs either
  a runner whose CLI exposes per-phase calls or a wrapper around Cursor
  that owns the surrounding shell steps; both are V2.
- **Multiple runners selected at runtime** — only Cursor in V1; the
  interface exists so adapters for Claude, Codex, etc. land as one new
  file each later.
- **Retry/backoff and failure taxonomy** — V2 of
  `AGENTIC-LAYER-PLAN.md`; one attempt per task in V1.
- **Task-level lock/claim, multi-replica safety** — V2/V4. V1 runs one
  worker per process; running two `taskapi` replicas with the worker
  enabled is **not supported** and the docs say so.
- **Per-replica Prometheus alerts and runbooks** — V3 of
  `AGENTIC-LAYER-PLAN.md`; Stage 6 only adds the counters / histograms
  the alerts will eventually consume.
- **Dead-letter handling** — V4.
- **Cycle REST routes (`POST /tasks/{id}/cycles`, etc.)** — already
  **shipped** in Stage 4 of `EXECUTION-CYCLES-PLAN.md`
  (commit `9151a58`), but the V1 worker still writes through the
  store directly to avoid an extra HTTP hop and double SSE fan-out
  inside the same process. External clients (UI, another worker)
  use the REST routes; the in-process V1 worker uses the store.
- **`task_cycle_changed` SSE event** — already **shipped** in Stage 5
  of `EXECUTION-CYCLES-PLAN.md` (commit `0b2be37`). The V1 worker
  becomes the first server-side publisher via the
  `CycleChangeNotifier` adapter wired in Stage 4 of this plan
  (edge case #6 + Stage 4 SSE adapter section). The SPA already
  invalidates the cycles cache slot granularly on receipt
  (Stage 7 of `EXECUTION-CYCLES-PLAN.md`, commit `d5948d2`).
- **Web UI execution panel** — Stage 8 of `EXECUTION-CYCLES-PLAN.md`,
  intentionally skipped during the substrate slice (mirror
  `task_events` already render via `TaskUpdatesTimeline`); promote
  once V1 worker activity in production demonstrates a need.
- **Cursor CLI security guardrails beyond env allowlist + redaction**
  — V2 of `AGENTIC-LAYER-PLAN.md`.
- **Splitting the worker into its own binary (`cmd/taskagent`)** — only
  worth doing once we want to scale the worker independently of
  `taskapi`. V1 stays in-process per `AGENTIC-LAYER-PLAN.md` V1.

## Notes / followups

(Populated as stages discover incidental work — keep this section as the
catch-all so individual stages stay scoped.)

The following edge cases were considered while writing this plan and
**intentionally deferred** out of V1. Each is documented so it cannot get
lost; promote into a follow-up plan once V1 is in production and we have
real signal about which one bites first.

- **Working-directory hygiene between sequential runs.** V1 has one
  worker, one working directory, sequential tasks. Cursor's whole job
  is to modify files; task A leaves edits, task B sees them. V1
  documents this as an operator responsibility in `docs/AGENT-WORKER.md`.
  V2 will need per-cycle workspace isolation (git worktrees, ephemeral
  clones, or branch-per-cycle).
- **Task cascade-delete mid-cycle.** If `DELETE /tasks/{id}` runs while
  the worker is mid-cycle, the next store call returns `ErrNotFound`.
  Worker handles this gracefully (Stage 3 test pins it) but a future
  enhancement should add a soft-delete grace window so an in-flight
  cycle can finish and write a final audit row.
- **Explicit byte caps on `RawOutput` and `Details`.** Stage 1 should
  pin these at construction (`RawOutput ≤ 64 KiB after redaction`,
  `Details ≤ 16 KiB`) with a `truncated: true` marker rather than
  failing. Captured here so the cap values are a one-line PR if Stage 1
  ships without them.
- **Redelivery between `AckAfterRecv` and `TerminateCycle`.** Made
  impossible in V1 by ack-after-terminate ordering (Stage 3, step 8).
  If the ordering is ever reversed for performance, the duplicate-cycle
  test from Stage 3 must be reinstated.
- **Prompt-secret redaction in slog.** V1 logs `prompt_hash` and
  prompt length, never the prompt body. Stage 6 should add a regression
  test that scans worker log output for the literal prompt string and
  fails the build if it appears. Same scan applies to the runner's
  `RawOutput` capture path.
- **Per-process clock skew on `StartedAt` / `EndedAt`.** Today these
  are `time.Now().UTC()`. NTP corrections can produce `EndedAt <
  StartedAt`. A monotonic-time wrapper or a `max(now, started)` clamp
  in `TerminateCycle` would prevent visibly wrong durations; cosmetic
  for now.
- **Cardinality of `t2a_agent_runs_total` once multi-runner lands.**
  Stage 6 ships labels `{runner, terminal_status}`. With one runner
  this is 4 series; with five runners it grows to 20. Still bounded,
  but worth re-checking in V2.

## Status

| Stage | State | Commit |
|---|---|---|
| 0 — Plan | done | `84083f4` (initial) + this commit (substrate-drift fixes) |
| 1 — Runner interface + fake | done | `f5c44b6` |
| 2 — Cursor CLI adapter | done | `14f5d17` |
| 3 — Worker loop | done | `06958a2` |
| 4 — `cmd/taskapi` wiring + config + startup sweep | done | `b775ab5` |
| 5 — Backend docs + contract pinning | done | _backfilled after commit_ |
| 6 — Observability + integration sweep | pending | — |
