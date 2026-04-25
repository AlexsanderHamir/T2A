# Agent worker (V1)

Authoritative behavior for the **single in-process Cursor CLI worker** that consumes the ready-task queue and drives one execution cycle per task. Versioned roadmap: [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md). Substrate the worker writes through: [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md). Queue it consumes: [AGENT-QUEUE.md](./AGENT-QUEUE.md). Architecture hub: [DESIGN.md](./DESIGN.md). Forward-looking design proposals: [`proposals/`](./proposals/).

## Why this exists

`pkgs/agents` ships ready-task snapshots into a bounded in-memory queue (`MemoryQueue`). Until V1 the queue had **no in-process consumer**: the only thing that dequeued tasks was the test suite, and operators ran agents externally against the REST API. The V1 worker is the first real consumer — a single goroutine that turns a queued `domain.Task` into one `task_cycle` row, one `execute` phase, the corresponding `task_events` audit mirrors, and a final task status transition (`done` or `failed`).

V1 is deliberately small:

- **One worker per process.** Concurrent processing of the same task is prevented by the store's "at most one running cycle per task" guard, not by a separate claim/lease (see [Limitations](#limitations)).
- **One runner.** Cursor CLI through `pkgs/agents/runner/cursor`. The `pkgs/agents/runner` interface exists so Claude Code, Codex, etc. land as one new file each in V2.
- **One attempt per task.** No retry/backoff; failure is terminal. V2 of [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) owns the retry policy.
- **One execute phase per cycle.** The `diagnose → execute → verify → persist` decomposition lives in the substrate's state machine but the worker only writes a `skipped` `diagnose` (to satisfy `domain.ValidPhaseTransition`) and a real `execute` phase. Per-phase decomposition is V2.

The worker is **enabled by default** as soon as a workspace repo is configured. Operators control everything from the SPA Settings page (gear icon → `/settings`), persisted in the singleton `app_settings` row — see [SETTINGS.md](./SETTINGS.md). When the page disables the worker (or the workspace repo is unset / the runner probe fails), `taskapi` behaves as it did before V1: the queue + reconcile loop run, but no worker dequeues and no cursor binary is required.

## Architecture

```mermaid
flowchart LR
  subgraph store["pkgs/tasks/store"]
    ST[(task_cycles<br/>task_cycle_phases<br/>task_events)]
  end

  subgraph queue["pkgs/agents"]
    NQ[Ready-task notifier]
    Q[MemoryQueue]
    RC[ReconcileLoop]
  end

  subgraph worker["pkgs/agents/worker"]
    SWEEP[SweepOrphanRunningCycles<br/>at startup]
    W[Worker.Run goroutine]
    W -->|StartCycle / StartPhase /<br/>CompletePhase / TerminateCycle| ST
  end

  subgraph runner["pkgs/agents/runner"]
    R[runner.Runner]
  end

  subgraph adapter["pkgs/agents/runner/cursor"]
    CUR[Adapter ⇒ exec cursor --print --output-format stream-json]
    PROBE[Probe ⇒ cursor --version<br/>at startup]
  end

  ST -->|notify on ready| NQ
  NQ --> Q
  RC -->|periodic backfill| Q
  Q -->|Receive| W
  W -->|Run\(req\)| R
  R --> CUR
  PROBE -.->|fail-fast at boot| W
  SWEEP -.->|idempotent cleanup| ST

  W -->|PublishCycleChange| HUB[handler.SSEHub]
  HUB -.->|task_cycle_changed| UI[Browser SSE]
```

Component map:

| Package | Role |
|---------|------|
| [`pkgs/agents/runner`](../pkgs/agents/runner) | `Runner`, `Request`, `Result` interface; typed sentinel errors (`ErrTimeout`, `ErrNonZeroExit`, `ErrInvalidOutput`). |
| [`pkgs/agents/runner/cursor`](../pkgs/agents/runner/cursor) | Cursor CLI adapter: `--print --output-format stream-json`, env allowlist, secret redaction, live progress normalization, `Probe(cursor --version)` for startup checks. |
| [`pkgs/agents/runner/runnerfake`](../pkgs/agents/runner/runnerfake) | Programmable in-memory `Runner` for unit + integration tests. |
| [`pkgs/agents/worker`](../pkgs/agents/worker) | `Worker`, `Options`, `CycleChangeNotifier` interface, `SweepOrphanRunningCycles`. |
| [`cmd/taskapi/run_helpers.go`](../cmd/taskapi/run_helpers.go) | Wires the four pieces above into `taskapi`'s lifecycle (probe → sweep → adapter → SSE notifier → `Worker.Run`). |

## Lifecycle of one task

`Worker.processOne` is intentionally written as a single top-down call site so the `defer` ordering — and the "ack only after terminate" invariant — can be read in one pass. The numbered steps below match the function body in `pkgs/agents/worker/worker.go`:

```mermaid
sequenceDiagram
  participant Q as MemoryQueue
  participant W as Worker.processOne
  participant S as Store
  participant R as runner.Runner
  participant N as CycleChangeNotifier

  Q->>W: Receive(task)
  W->>S: Get(task.ID) (reload)
  Note right of W: stale? log Warn, ack, return
  W->>S: Update(task, status=running)
  W->>S: StartCycle(task, meta={runner, runner_version, prompt_hash})
  S-->>W: cycle (task_cycles row + cycle_started mirror)
  W->>N: PublishCycleChange(taskID, cycleID)
  W->>S: StartPhase(cycle, diagnose) → CompletePhase(skipped, "single-phase V1; diagnose deferred")
  W->>N: PublishCycleChange × 2
  W->>S: StartPhase(cycle, execute)
  W->>N: PublishCycleChange
  W->>R: Run(ctx, Request{TaskID, AttemptSeq, Phase=execute, Prompt, WorkingDir, Timeout})
  R-->>W: (Result, error)
  W->>S: CompletePhase(execute, succeeded|failed, summary, details)
  W->>N: PublishCycleChange
  W->>S: TerminateCycle(cycle, succeeded|failed|aborted, reason)
  W->>N: PublishCycleChange
  W->>S: Update(task, status=done|failed)
  W->>Q: AckAfterRecv(task.ID)  (very last step)
```

Order is load-bearing:

- **`AckAfterRecv` runs last (deferred, LIFO).** Until terminate succeeds the task id stays in the queue's pending set, so a notify+reconcile race that re-enqueues the same id during a long-running attempt is rejected (`agents.ErrAlreadyQueued`) instead of producing a second `StartCycle`.
- **Panic recovery** runs **before** ack (the deferred order is `defer w.queue.AckAfterRecv(...)` then `defer w.recoverFromPanic(...)`, so recover runs first). Recover writes a best-effort `CompletePhase(failed, "panic")` + `TerminateCycle(failed, "panic")` + `Update(task, failed)` on a fresh background context with a 5s deadline so the audit trail still lands when the runner explodes.
- **Shutdown branch** runs after `runner.Run` returns, before `CompletePhase`. If `parentCtx.Err() != nil` the worker writes `CompletePhase(failed, "shutdown")` + `TerminateCycle(aborted, "shutdown")` + `Update(task, failed)` on a 5s background context and returns. The startup orphan sweep is the safety net if even that 5s budget trips (see [Process restart and the orphan sweep](#process-restart-and-the-orphan-sweep)).

## Runner abstraction

`runner.Runner` is the seam between the worker and any agent CLI. The interface is intentionally narrow so the worker has no opinion about argv, env, or output parsing:

```go
// pkgs/agents/runner/runner.go
type Runner interface {
    Run(ctx context.Context, req Request) (Result, error)
    Name() string
    Version() string
}

type Request struct {
    TaskID     string
    AttemptSeq int64
    Phase      domain.Phase
    Prompt     string
    WorkingDir string
    Timeout    time.Duration
    Env        map[string]string
}

type Result struct {
    Status    domain.PhaseStatus
    Summary   string
    Details   json.RawMessage
    RawOutput string
    Truncated bool
}
```

Errors must be wrapped with one of the typed sentinels so the worker's `classifyRunOutcome` can map them onto the right cycle/task status:

| Sentinel | When | Mapped phase status | Mapped cycle status | Mapped task status | `reason` written to `cycle_failed` mirror |
|---|---|---|---|---|---|
| `nil` | clean success | `succeeded` | `succeeded` | `done` | (none) |
| `runner.ErrTimeout` | runner saw `ctx.Done()` (per-run timeout or process shutdown) | `failed` | `failed` | `failed` | `runner_timeout` |
| `runner.ErrNonZeroExit` | child process exited with non-zero code | `failed` | `failed` | `failed` | `runner_non_zero_exit` |
| `runner.ErrInvalidOutput` | stdout could not be parsed into `Result`, or the child failed to start (cursor adapter wraps `os/exec` start errors here too) | `failed` | `failed` | `failed` | `runner_invalid_output` |
| any other error | unexpected adapter failure not covered above | `failed` | `failed` | `failed` | `runner_error` |

`Name()` and `Version()` are recorded once per cycle in `task_cycles.meta_json`:

```json
{
  "runner": "cursor-cli",
  "runner_version": "<output of cursor --version>",
  "prompt_hash": "<sha256(initial_prompt) hex>"
}
```

`prompt_hash` is the hash of the task's `initial_prompt`, never the prompt body — the body could leak secrets and the audit trail is forever (see [Security model](#security-model)).

V1 ships exactly one adapter; V2 multi-runner selection is intentionally deferred (see [`AGENTIC-LAYER-PLAN.md`](./AGENTIC-LAYER-PLAN.md) V2).

## Cursor adapter

`pkgs/agents/runner/cursor` is the production `Runner` implementation.

**Invocation contract:**

- `cursor --print --output-format stream-json` (overridable via `cursor.Options.Args`).
- The task's `initial_prompt` is fed on stdin.
- Working directory is `runner.Request.WorkingDir`, which the worker fills from `app_settings.repo_root` (see [Configuration](#configuration) and [SETTINGS.md](./SETTINGS.md)).
- Timeout is `runner.Request.Timeout`, which the worker fills from `app_settings.max_run_duration_seconds` (default `0` = no limit). When non-zero, the adapter applies `context.WithTimeout` on top of the worker's ctx; either firing maps to `runner.ErrTimeout`.

**Env allowlist (defense in depth):**

- The child process inherits **only** `PATH`, `HOME`, `USERPROFILE` from the parent env, plus any keys passed in `runner.Request.Env`.
- A hardcoded deny-list scrubs `DATABASE_URL` and any `T2A_*` key even if the caller adds it via `Options.ExtraAllowedEnvKeys`. Cursor never sees the store credentials or the worker's own configuration.

**Output redaction:**

- `RawOutput` is the combined stdout + stderr of the child, post-redaction, capped before being persisted into `task_cycle_phases.details_json`.
- The redactor (`cursor.Redact`) replaces `Authorization: …` lines with `Authorization: [REDACTED]`, replaces any `T2A_…=value` assignment with `T2A_…=[REDACTED]`, and rewrites absolute home paths (`$HOME`, `$USERPROFILE`) to `~` so log scrapers do not learn the operator's local layout.

**Live progress:**

- The adapter reads Cursor `stream-json` stdout line-by-line while the child process is still running. The terminal `result` event is still parsed into `runner.Result` for the durable phase row.
- Intermediate `system.init`, `assistant`, and `tool_call` events are normalized into small `runner.ProgressEvent` values and forwarded through `runner.Request.OnProgress`. Raw Cursor JSON and stderr are not sent to the browser.
- The worker publishes those updates as ephemeral `agent_run_progress` SSE frames keyed by task id, cycle id, and phase sequence. They are throttled before fanout and are not written to `task_events`; the next `task_cycle_changed` frame and REST refetch remain authoritative for audit/history.

**Startup probe:**

- Whenever `app_settings.worker_enabled` is true and the supervisor decides it can start the worker, it calls `cursor.Probe(ctx, cursorBin, 5s, nil)` before the worker loop spins up. The probe shells out `<cursorBin> --version` and uses the trimmed first non-empty line of stdout (or stderr) as the `Runner.Version()` value. The same probe runs on every `POST /settings/probe-cursor` so the SPA "Test cursor binary" button uses identical logic — see [SETTINGS.md](./SETTINGS.md).
- Probe failure (binary missing on `PATH`, non-zero exit, exec error, timeout, empty output) logs `Error("cursor binary not usable, refusing to start agent worker", …)` and **exits 1** per the engineering bar's "fail loudly at startup" rule. Operators see the failure in the same boot log they would already be reading; the alternative — failing per-task hours later — is harder to triage.
- The probe is **not** run when the worker is disabled, so operators without Cursor CLI on `PATH` are unaffected by the V1 wiring.

## Audit trail (one happy-path attempt)

For one successful task the worker produces:

- **`task_cycles`**: 1 row, `attempt_seq=1`, `status=succeeded`, `meta_json={runner, runner_version, prompt_hash}`.
- **`task_cycle_phases`**: 2 rows.
  - Phase 1: `phase=diagnose`, `phase_seq=1`, `status=skipped`, `summary="single-phase V1; diagnose deferred"`, `details_json={}`.
  - Phase 2: `phase=execute`, `phase_seq=2`, `status=succeeded`, `summary` and `details_json` from `runner.Result`.
- **`task_events`** (mirrors, all in the same SQL transactions as the cycle/phase writes — see [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) "Dual-write invariant"):

| Seq (illustrative) | Type | Trigger |
|---|---|---|
| n+1 | `cycle_started` | `StartCycle` |
| n+2 | `phase_started` | `StartPhase(diagnose)` |
| n+3 | `phase_skipped` | `CompletePhase(diagnose, skipped)` |
| n+4 | `phase_started` | `StartPhase(execute)` |
| n+5 | `phase_completed` | `CompletePhase(execute, succeeded)` |
| n+6 | `cycle_completed` | `TerminateCycle(succeeded)` |

Plus the regular `status_changed` events that `(*store.Store).Update` writes when the task transitions `ready → running` and then `running → done`, exactly as for any other actor that flips a task through the REST API.

The worker also publishes one `task_cycle_changed` SSE event per cycle/phase mutation (six per happy-path attempt) via the `CycleChangeNotifier` adapter wired in `cmd/taskapi/run_helpers.go`. The SPA routes that event to its dedicated cycles cache slot (see [API-SSE.md](./API-SSE.md)), so cycle activity appears in any open browser without a page refresh and without the SPA refetching the entire task tree. While the execute phase is running, normalized Cursor stream events may also publish `agent_run_progress` frames so the UI can show recent agent activity between `phase_started` and `phase_completed`.

Failure mirrors are symmetrical: a failed runner produces `phase_failed` (instead of `phase_completed`) and `cycle_failed` (instead of `cycle_completed`), with the `reason` field in the `cycle_failed` payload set to one of the strings in the [Runner abstraction table](#runner-abstraction). A panic produces the same shape with `reason="panic"`. Shutdown produces `cycle_failed` with `status=aborted` and `reason="shutdown"`.

## Process restart and the orphan sweep

If `taskapi` is killed mid-cycle (OS kill, deadline trip on the 5s shutdown budget, power loss), the `task_cycles.status='running'` and `task_cycle_phases.status='running'` rows from the in-flight attempt are stuck — they cannot resolve themselves and the store's "at most one running cycle per task" guard would block any new attempt forever.

`worker.SweepOrphanRunningCycles(ctx, st)` is the safety net. It runs **once at startup**, **before** `Worker.Run` begins, and only when the supervisor decides the worker can run (see [SETTINGS.md](./SETTINGS.md) — `worker_enabled=true`, `repo_root` set, runner probe ok):

1. List every `task_cycle_phases` row with `status='running'`. For each: `CompletePhase(failed, "process_restart")`. Phase-first order avoids the "cycle has running phase" guard inside `TerminateCycle`.
2. List every `task_cycles` row with `status='running'`. For each: `TerminateCycle(aborted, "process_restart")` (writes a `cycle_failed` mirror with `status: aborted` and `reason: process_restart`).
3. For each cycle aborted by step 2 whose underlying task is still in `StatusRunning`: `Update(task, status=failed)`. Tasks in any other status are left alone — the sweep never overwrites manual or REST-driven state.

The sweep is idempotent: re-running on a clean DB is a no-op and reports `{cycles_aborted: 0, phases_failed: 0, tasks_failed: 0}`. Errors on individual rows are logged and skipped so one bad row cannot block the sweep; the result counters reflect rows actually mutated. The startup log line is `agent worker startup sweep ok` with the three counts.

When the worker is disabled, the sweep is **not** run — leaving any running rows alone is intentional, since they may have been written by a worker enabled in a previous boot or by an external client that owns its own lifecycle.

## Configuration

All V1 worker knobs live in the singleton `app_settings` DB row and are surfaced on the SPA Settings page (`/settings`). Authoritative reference: [SETTINGS.md](./SETTINGS.md). The supervisor reloads them in-process whenever `PATCH /settings` succeeds, so changes never require a restart.

| Field | Default | Effect |
|-------|---------|--------|
| `worker_enabled` | `true` | Turn the in-process worker off without restarting `taskapi`. |
| `runner` | `cursor` | Runner identifier from the `pkgs/agents/runner/registry`. Currently only `cursor` ships. |
| `repo_root` | empty | Absolute path to the workspace the worker (and `/repo/*`) operates against. While empty the supervisor stays idle. |
| `cursor_bin` | `cursor` (resolved against `PATH`) | Cursor CLI binary used by both the startup probe and the runner. Absolute path pins a build; relative names go through `PATH`. |
| `max_run_duration_seconds` | `0` (no limit) | Per-run wall-clock cap forwarded to `runner.Request.Timeout`. `0` means "no limit"; positive values are honoured exactly. |

Related queue/reconcile env vars (always read, even when the worker is idle):

- `T2A_USER_TASK_AGENT_QUEUE_CAP` — buffer depth of the `MemoryQueue` the worker dequeues from. See [AGENT-QUEUE.md](./AGENT-QUEUE.md).
- Reconcile tick interval is fixed in code (`pkgs/agents.ReconcileTickInterval`, 2 minutes); it is not an env var. It backfills ready tasks the notifier dropped.

The supervisor emits one structured line on every (re)load summarizing the live config:

```
slog.Info("agent worker supervisor reload", "cmd"="taskapi", "operation"="taskapi.agent_worker_supervisor",
  "enabled"=true,
  "runner"="cursor",
  "cursor_bin"="cursor",
  "cursor_version"="cursor 1.2.3",
  "run_timeout_sec"=0,
  "repo_root"="/home/op/code/myrepo")
```

When the supervisor stays idle the same line is emitted with `enabled=false` (or a non-empty `reason` such as `repo_root_not_configured` / `probe_failed`) so log scrapers can tell "operator disabled it" apart from "supervisor refused to start it."

## Security model

The worker runs Cursor CLI as a child process inside the same user account as `taskapi`. The defenses listed below are the V1 floor; V2 of [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) adds a second layer (sandboxing, prompt-injection guards, output validation).

- **Env allowlist:** the child sees only `PATH`, `HOME`, `USERPROFILE`, plus keys explicitly allowed via `cursor.Options.ExtraAllowedEnvKeys`. `DATABASE_URL` and any `T2A_*` key are scrubbed unconditionally — Cursor cannot read the store credentials or the worker's own configuration even if a future code path tries to forward them.
- **Secret redaction in `RawOutput`:** the redactor (`cursor.Redact`) is applied before the combined stdout + stderr is written to `task_cycle_phases.details_json`. It blanks `Authorization: …` headers, `T2A_…=value` assignments, and rewrites absolute home paths to `~`.
- **Prompt hashing in audit:** `task_cycles.meta_json.prompt_hash` records `sha256(initial_prompt)`, never the prompt body. The body lives only on `tasks.initial_prompt` (where it was already authored) and is forwarded to the child on stdin; it never enters the audit log or worker `slog` lines.
- **Per-run wall-clock cap:** when `app_settings.max_run_duration_seconds > 0`, every `runner.Run` is wrapped in `context.WithTimeout(parentCtx, max_run_duration)`. The default (`0`) is "no limit" — runs only end on completion, operator cancel, or process shutdown. Operators raise/lower the cap from the SPA Settings page; see [SETTINGS.md](./SETTINGS.md).
- **Working-dir hygiene is the operator's job (V1).** V1 has one worker, one working directory, sequential tasks. Cursor's whole job is to modify files; task A leaves edits, task B sees them. V2 will need per-cycle workspace isolation (git worktrees, ephemeral clones, branch-per-cycle) — see V2 in [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md).
- **No outbound network policy enforcement.** V1 trusts that Cursor CLI honors its own configured network behavior. V2 may add a network namespace / proxy-only policy for the child process.

## Composing with the queue and the cycles substrate

The worker is a **consumer**, not a co-author of the surrounding contracts:

- **Queue (`pkgs/agents` / [AGENT-QUEUE.md](./AGENT-QUEUE.md)):** the worker calls `(*MemoryQueue).Receive` and `AckAfterRecv` and never touches the pending set otherwise. Reconcile keeps backfilling ready tasks even while the worker is running, so a startup dropped notify cannot leak.
- **Cycles substrate (`pkgs/tasks/store` / [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md)):** the worker writes through the same `StartCycle` / `StartPhase` / `CompletePhase` / `TerminateCycle` facade methods that external clients use through the REST routes. The dual-write invariant (every cycle/phase mutation appends a `task_events` mirror in the same SQL transaction) is enforced by the store, not the worker — the worker just calls the facade and lets the substrate do its work.
- **REST routes vs. in-process write:** `POST /tasks/{id}/cycles` (and friends — see [API-HTTP.md](./API-HTTP.md)) remain the contract for **external** clients. The V1 worker bypasses HTTP and writes directly through the store to avoid an extra HTTP hop and double SSE fan-out inside the same process; this is documented out-of-scope for the REST routes.
- **SSE (`task_cycle_changed`):** the contract lives in [API-SSE.md](./API-SSE.md). The V1 worker is the first server-side publisher via the `cycleChangeSSEAdapter` in `cmd/taskapi/run_helpers.go`. The SPA invalidates the cycles cache slot granularly on receipt.

What the worker does **not** know about:

- Task DAG edges (`parent_id`, `subtask_added` events) — it processes one task at a time and the parent/child relationship is unchanged by an attempt.
- Checklists, drafts, evaluations — the worker is not currently allowed to create them.
- The `T2A_API_TOKEN` HTTP auth — the worker writes through the store, not HTTP.

## Limitations

1. **One worker per process.** Running two `taskapi` replicas with the worker enabled is **not supported**: the orphan sweep on replica B at startup would race replica A's in-flight cycles. Multi-replica safety is V2/V4 of [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md).
2. **One attempt per task.** A failed runner sets the task to `failed`; nothing in V1 tries again. To re-run, an operator (or future retry policy) flips the task back to `ready`.
3. **One execute phase per cycle.** The substrate's `diagnose / verify / persist` phases exist, but the worker writes only `skipped diagnose` + `execute` per cycle. Per-phase decomposition is V2.
4. **In-process queue, not durable.** The `MemoryQueue` lives in RAM. A crash drops every queued snapshot; reconcile re-enqueues from the database on the next boot, so no work is permanently lost, but the dedupe pending set restarts empty.
5. **Working directory is shared across sequential runs.** V1 trusts that consecutive tasks tolerate prior edits; V2 will isolate each cycle.
6. **No retry/backoff, no failure taxonomy.** All non-nil runner errors map to `cycle_failed` + `task=failed` with one of five fixed `reason` strings; there is no notion of "transient vs terminal" yet.
7. **No claim/lease at the task level.** Concurrency is prevented by the store's "at most one running cycle per task" guard, which works for one in-process worker but does not generalize to multiple replicas. V4 picks one of database lease vs external broker.
8. **Best-effort secret hygiene.** The env allowlist + `cursor.Redact` are the V1 floor; the redactor is regex-based and a malicious / sloppy CLI plugin can still print bytes that trip nothing in the deny list. V2 adds a second guard layer.

## Operator quick-look

What an operator sees in logs and in `task_events` for one happy-path attempt of `task_id=abc`:

**`slog` lines (info level, abridged):**

```
{"msg":"agent worker config", "enabled":true, "runner":"cursor-cli", "cursor_version":"cursor 1.2.3", "run_timeout_sec":300, "working_dir":"/home/op/repo"}
{"msg":"agent worker startup sweep ok", "cycles_aborted":0, "phases_failed":0, "tasks_failed":0}
... (worker dequeues task abc) ...
{"msg":"agent worker run complete",
 "task_id":"abc", "cycle_id":"cyc-…", "attempt_seq":1,
 "terminal_cycle_status":"succeeded", "task_status":"done",
 "runner":"cursor-cli", "runner_version":"cursor 1.2.3",
 "duration_ms":4231}
```

**`task_events` (newest last):**

```
… status_changed (ready → running)
   cycle_started
   phase_started (diagnose)
   phase_skipped (diagnose, summary="single-phase V1; diagnose deferred")
   phase_started (execute)
   phase_completed (execute)
   cycle_completed
   status_changed (running → done)
```

A failed attempt swaps `phase_completed` → `phase_failed`, `cycle_completed` → `cycle_failed` (with `reason` payload), and `status_changed (running → failed)`. A panic adds `reason="panic"`. A shutdown adds `reason="shutdown"` and the cycle ends in `aborted` instead of `failed`.

### Smoke run (operator-only, real cursor-agent)

The fake-runner test suite covers every wiring decision in V1, but it cannot prove the wired-up system actually drives a real Cursor CLI invocation to completion. That gap is closed by the **real-cursor smoke test**, shipped in two layers:

| Layer | What it proves | File |
|-------|----------------|------|
| Runner-only | The `cursor.Adapter` correctly invokes `cursor-agent`, parses its JSON envelope, and the working directory ends up with the expected file. | [`pkgs/agents/runner/cursor/cursor_real_smoke_test.go`](../pkgs/agents/runner/cursor/cursor_real_smoke_test.go) |
| Full flow | A `POST /tasks` with `status=ready` flows through reconcile → worker → real `cursor-agent`, the cycle/phase audit lands correctly, the file is on disk, the SSE hub emits `task_cycle_changed`, and the Prometheus metrics record exactly one `succeeded` run. | [`pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go`](../pkgs/tasks/agentreconcile/agent_real_cursor_e2e_test.go) |

**When to run.** Any change to `pkgs/agents/runner/cursor/`, the `pkgs/agents/worker/` happy path, or the wiring in `cmd/taskapi/run_agentworker.go`. CI does not run these tests because CI does not have a Cursor login; the smoke is an operator-run gate before merging changes that touch those areas.

**Prerequisites.**

- `cursor-agent` installed and on `PATH` (or set the absolute path on the SPA Settings page → "Cursor binary"; the Windows shim is `cursor-agent.cmd`).
- Cursor logged in for the local user account that runs the test.
- Both stages take ~12–14 s wall-clock against a warm Cursor session; budget 60 s on a cold cache.

**How to run.**

```powershell
# Runner-only smoke (cursor.Adapter against the live binary)
$env:T2A_TEST_REAL_CURSOR='1'
go test -tags=cursor_real -run TestCursorAdapter_RealBinary `
    ./pkgs/agents/runner/cursor/... -count=1

# Full flow (HTTP -> worker -> cursor-agent -> file on disk)
$env:T2A_TEST_REAL_CURSOR='1'
go test -tags=cursor_real -run TestAgentE2E_RealCursor `
    ./pkgs/tasks/agentreconcile/... -count=1
```

```bash
# bash equivalent
T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -run TestCursorAdapter_RealBinary \
    ./pkgs/agents/runner/cursor/... -count=1

T2A_TEST_REAL_CURSOR=1 go test -tags=cursor_real -run TestAgentE2E_RealCursor \
    ./pkgs/tasks/agentreconcile/... -count=1
```

Both tests are **double-gated**: they no-op without the `cursor_real` build tag *and* without `T2A_TEST_REAL_CURSOR=1`, so a stray `go test ./...` can never trigger a paid Cursor run.

**When the smoke fails.** The tests print operator-readable failure context before exiting:

- `cursor probe failed` → `cursor-agent` is missing or not executable. Install it or open the SPA Settings page (gear icon → "Cursor binary") and set the absolute path of the binary, then click "Test cursor binary" (Windows: typically `C:\Users\<you>\AppData\Local\cursor-agent\cursor-agent.cmd`).
- `task ... final status = "failed"` → the test dumps the cycle's `MetaJSON` and per-phase `Summary` + `DetailsJSON` tail. Check Cursor login (`cursor-agent --version` should succeed without a login prompt) and inspect the `details_tail` for the redacted CLI output.
- `unexpected extra files` warnings (informational only) → on Windows, OS-level cache files (`cversions.2.db`, `*.ver*`) sometimes drop into the test's temp working directory. The harness logs these and continues; the only authoritative assertion is the target file's contents.
- Test wall-clock above 90 s → Cursor cold-cache or a network blip. Re-run; if persistent, raise it locally first before opening an issue.

The deterministic prompt shape, gating strategy, and the adapter bugs surfaced during the smoke's rollout are captured in the test files themselves and in `pkgs/agents/agentsmoke/doc.go`.

## Metrics

The worker exposes two Prometheus series, registered by `taskapi.RegisterAgentWorkerMetrics()` in `internal/taskapi/agent_worker_metrics.go` and observed through the `worker.RunMetrics` interface so the worker package itself does not depend on Prometheus:

| Series | Type | Labels | Source |
|--------|------|--------|--------|
| `t2a_agent_runs_total` | counter | `runner`, `terminal_status` | Incremented exactly once per `TerminateCycle` write (happy path, panic, shutdown abort, best-effort intermediate failure). |
| `t2a_agent_run_duration_seconds` | histogram | `runner` | Observes `now - state.startedAt` at the same call site, with buckets tuned for the V1 run-timeout range (`0.5s … 30m`). |
| `t2a_agent_runs_by_model_total` | counter | `runner`, `model`, `terminal_status` | **Parallel series** emitted from the same `RecordRun` call as `t2a_agent_runs_total`. `model` is `runner.Runner.EffectiveModel(req)` resolved at cycle start — the concrete model the runner actually executed against (see [EXECUTION-CYCLES.md § Cycle metadata](./EXECUTION-CYCLES.md#cycle-metadata-meta_json--cycle_meta)). Empty string is recorded verbatim and means "no model configured". |
| `t2a_agent_run_duration_by_model_seconds` | histogram | `runner`, `model` | Parallel histogram companion to the `by_model` counter. Same buckets as `t2a_agent_run_duration_seconds`. |

**Parallel series, not replacement.** The `_by_model_` companions were added for per-model dashboards without breaking any existing query or alert: `t2a_agent_runs_total{runner,terminal_status}` and `t2a_agent_run_duration_seconds{runner}` remain byte-identical. Every call to `RecordRun` emits both the original and the `_by_model_` observation, so the sums across models reconcile exactly with the original per-runner totals (modulo label-set differences). This additivity is tested in `internal/taskapi/agent_worker_metrics_test.go`. The cost is one extra `Inc()` + `Observe()` per terminated cycle, negligible.

**Cardinality.** `runner` comes from `runner.Runner.Name()` (today: `cursor-cli`, `fake` in tests) and `terminal_status` is one of the three terminal `domain.CycleStatus` values (`succeeded`, `failed`, `aborted`). `model` is **not** capped at the wire — it ships verbatim as the effective model string. In practice fewer than ten Cursor models are in use simultaneously; watch `count({__name__="t2a_agent_runs_by_model_total"})` if you add a new runner family, and document any expected ceiling alongside it.

The seam itself is `worker.RunMetrics`:

```go
// pkgs/agents/worker/metrics.go
type RunMetrics interface {
    RecordRun(runner string, model string, terminalStatus string, duration time.Duration)
}
```

`Worker.Options{Metrics: ...}` is the wiring point. Tests pass `nil` (no observation), production wires `taskapi.RegisterAgentWorkerMetrics()` which registers on the default Prometheus registry exactly once and returns the adapter. Implementations MUST NOT block: the worker invokes `RecordRun` synchronously after each `TerminateCycle` write, so a slow sink would back-pressure the run loop.

The companion queue gauges (`taskapi_agent_queue_depth`, `taskapi_agent_queue_capacity`) ship in `internal/taskapi/agent_queue_metrics.go` and are independent of the worker — they remain useful even when the worker is disabled, since reconcile is still doing the dequeue work for external clients.

Alert rules and runbooks for these series are deliberately deferred to V3 of [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md); V1 ships the raw observability surface so operators can build alerts on real production traffic rather than guessed thresholds.

## What's deliberately out of scope (V1)

Tracked under V2/V3/V4 of [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md); fresh design work for any of the items below should land as a proposal in [`proposals/`](./proposals/) before implementation begins.

- Per-phase decomposition (`diagnose / verify / persist` substance).
- Multi-runner selection at runtime.
- Retry/backoff and failure taxonomy.
- Multi-replica claim/lease and dead-letter handling.
- Per-cycle workspace isolation (git worktrees etc).
- Prometheus alert rules + runbooks (V1 ships the raw counter + histogram; alerts and runbooks remain V3 of `AGENTIC-LAYER-PLAN.md`).
- A standalone `cmd/taskagent` binary — only worth doing once the worker needs to scale independently of `taskapi`.

## Related

- [AGENT-QUEUE.md](./AGENT-QUEUE.md) — the `MemoryQueue` + reconcile loop the worker consumes from.
- [EXECUTION-CYCLES.md](./EXECUTION-CYCLES.md) — `task_cycles` / `task_cycle_phases` substrate the worker writes through, dual-write invariant.
- [RUNTIME-ENV.md](./RUNTIME-ENV.md) — full env table including the V1 worker variables.
- [API-SSE.md](./API-SSE.md) — `task_cycle_changed` SSE event the worker publishes through.
- [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md) — long-term roadmap (V0–V4) that V1 sits inside.
- [DESIGN.md](./DESIGN.md) — architecture hub, contract docs table, system limitations.
