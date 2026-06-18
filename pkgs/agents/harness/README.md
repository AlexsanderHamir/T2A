# pkgs/agents/harness

Cycle choreography around `runner.Run`. The worker (`pkgs/agents/worker`) handles queue admission; the harness drives one task from `StartCycle` through terminal `TerminateCycle`, or resumes an open cycle after process restart.

See [docs/architecture.md](../../docs/architecture.md) (Agent worker and harness), [docs/domain/execute-agent.md](../../docs/domain/execute-agent.md) (execute phase deep-dive), [docs/domain/verify-agent.md](../../docs/domain/verify-agent.md) (verify phase deep-dive), [ADR-0005](../../docs/adr/ADR-0005-extract-agent-harness.md), and [ADR-0006](../../docs/adr/ADR-0006-phase-boundary-resume.md).

## File map

| File | Responsibility |
|------|----------------|
| `harness.go` | `Harness`, `New`, `Options`, `CancelCurrentRun`, SSE notifiers, metrics interface |
| `cycle.go` | `Run` entry — starts a new cycle then delegates to the shared loop |
| `cycle_loop.go` | Shared execute/verify loop used by `Run` and `Resume` — see [docs/domain/execute-agent.md](../../docs/domain/execute-agent.md) |
| `resume.go` | `Resume` — continue an open cycle after `process_restart` finalization |
| `resume_state.go` | `reconstructCheckpoint` from phase ledger + report tables |
| `resume_prompt.go` | Resume notice, execute commit policy, verify clean-tree hints — see [docs/domain/execute-agent.md](../../docs/domain/execute-agent.md) |
| `verification.go` | Verify pipeline, LLM verify agent, criteria/verify report persistence — see [docs/domain/verify-agent.md](../../docs/domain/verify-agent.md) |
| `verify_integrity.go` | Pre/post git snapshot; tamper detection during verify |
| `criteria_prompt.go` | Criteria injection and verify feedback in execute prompts — see [docs/domain/execute-agent.md](../../docs/domain/execute-agent.md) |
| `criteria_parse.go` | Report file paths, parse `criteria-report.json` / `verify-report.json` |
| `project_context.go` | Project context selection and prompt injection (loads existing snapshot per cycle) |
| `meta.go` | Cycle `MetaJSON` and phase `details_json` normalization |
| `metrics.go` | `RunMetrics` seam and observation helpers |
| `recovery.go` | Panic, shutdown, and best-effort cycle closeout paths |

## Public entry points

```go
h := harness.New(store, runner, harness.Options{...})
h.Run(ctx, task)    // task must be StatusReady → worker sets running and starts new cycle
h.Resume(ctx, task, cycle) // task StatusRunning, cycle StatusRunning — same attempt continues
```

Callers outside tests typically use `worker.NewWorker`, which constructs the harness internally and chooses `Run` vs `Resume` at admission.

## Checkpoint derivation (resume)

No dedicated checkpoint table. `reconstructCheckpoint` reads:

- Phase ledger tail → execute vs verify resume branch
- `task_cycle_verify_reports` → locked passes, verify attempt, retry feedback
- Task row → base prompt
- `task_context_snapshots` for `cycle_id` → project context block
- Git working tree / `t2a:cycle=<cycle_id>` commits when commit policy is on

The composed prompt is what the runner sees; `WorkingDir` remains `app_settings.repo_root`.
