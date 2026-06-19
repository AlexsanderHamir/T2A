# pkgs/agents/harness

Cycle choreography around `runner.Run`. The worker (`pkgs/agents/worker`) handles queue admission; the harness drives one task from `StartCycle` through terminal `TerminateCycle`, or resumes an open cycle after process restart.

**Behavioral reference:** [docs/domain/harness.md](../../docs/domain/harness.md). See also [docs/architecture.md](../../docs/architecture.md), [docs/domain/execute-agent.md](../../docs/domain/execute-agent.md), [docs/domain/verify-agent.md](../../docs/domain/verify-agent.md), [ADR-0005](../../docs/adr/ADR-0005-extract-agent-harness.md), [ADR-0006](../../docs/adr/ADR-0006-phase-boundary-resume.md), [ADR-0017](../../docs/adr/ADR-0017-harness-internal-domains.md), [ADR-0018](../../docs/adr/ADR-0018-harness-orchestration-fsm.md), [ADR-0021](../../docs/adr/ADR-0021-harness-execute-orchestration.md), and [ADR-0028](../../docs/adr/ADR-0028-in-cycle-verify-only-retry.md).

## Internal layout

Domain logic lives under `internal/` (importable only from `harness` and sibling `internal/*`):

| Package | Role |
|---------|------|
| [`internal/reports/`](internal/reports/) | Side-channel JSON paths, parse/validate, `schema_version` |
| [`internal/git/`](internal/git/) | Commits, reset, verify integrity (`GitRepo` port) |
| [`internal/prompt/`](internal/prompt/) | Execute/verify prompt assembly |
| [`internal/verify/`](internal/verify/) | Verification pipeline stages |
| [`internal/resume/`](internal/resume/) | Checkpoint load, retry routing, continuation bundles |
| `internal/orchestration/` | Pure cycle Decide functions (`DecideVerifyRetry`, `DecideVerifyRetryWithValidity`, `ClassifyVerifyRetryMode`, `DecideExecutePostRun`, loop-level finalize/legacy) |

Root `harness` owns `Harness`, cycle entrypoints, effect application (`cycle_effects.go`), recovery, and metrics.

## File map (root package)

| File | Responsibility |
|------|----------------|
| `harness.go` | `Harness`, `New`, `Options`, `CancelCurrentRun`, SSE notifiers, metrics interface |
| `cycle.go` | `Run` entry — starts a new cycle then delegates to the shared loop |
| `cycle_loop.go` | Shared execute/verify loop coordinator; I/O then orchestration Decide |
| `cycle_effects.go` | Applies orchestration effects (store writes, publish, metrics) |
| `cycle_execute_adapter.go` | Maps runner/git facts to orchestration DTOs at the I/O boundary |
| `verify_retry_eligibility.go` | Post-execute anchors + `gatherRetryClassifyInput` (ADR-0028) |
| `cycle_verify_only_test.go` | Integration tests for in-cycle verify-only retry (EC-xx) |
| `resume.go` | `Resume` — continue an open cycle after `process_restart` finalization |
| `retry_run.go` | `RunWithRetry` — operator fresh/resume retry modes |
| `verification.go` | Thin delegators to `internal/verify` |
| `git_alias.go` | Thin delegators to `internal/git` |
| `resume_alias.go` | Thin delegators to `internal/resume` |
| `reports_alias.go` | Re-exports report sentinel errors |
| `project_context_load.go` | Loads store data for prompt assembly via `internal/prompt` |
| `execute_criteria_mirror.go` | Best-effort criteria mirror after execute |
| `meta.go` | Cycle `MetaJSON` and phase `details_json` normalization |
| `metrics.go` | `RunMetrics` seam and observation helpers |
| `effective_model.go` | Model resolution for execute/verify runners |
| `recovery.go` | Panic, shutdown, and best-effort cycle closeout paths |
| `invariant_test.go` | Durability/orchestration contract tests |

## Public entry points

```go
h := harness.New(store, runner, harness.Options{...})
h.Run(ctx, task)    // task must be StatusReady → worker sets running and starts new cycle
h.Resume(ctx, task, cycle) // task StatusRunning, cycle StatusRunning — same attempt continues
```

Callers outside tests typically use `worker.NewWorker`, which constructs the harness internally and chooses `Run` vs `Resume` at admission.

## Checkpoint derivation (resume)

No dedicated checkpoint table. `internal/resume` reconstructs checkpoint from:

- Phase ledger tail → execute vs verify resume branch
- `task_cycle_verify_reports` → locked passes, verify attempt, retry feedback
- Task row → base prompt
- `task_context_snapshots` for `cycle_id` → project context block
- `task_cycle_commits` → worker-indexed SHAs for resume/verify prompts (see [cycle-commits.md](../../docs/domain/cycle-commits.md))

The composed prompt is what the runner sees; `WorkingDir` remains `app_settings.repo_root`.
