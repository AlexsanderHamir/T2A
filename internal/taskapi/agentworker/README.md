# `internal/taskapi/agentworker`

In-process agent worker supervisor: settings-driven boot and hot reload, runner registry build/probe, instance lifecycle, SSE notifier wiring, and shutdown drain.

**Behavioral reference:** [docs/domain/agent-supervisor.md](../../../docs/domain/agent-supervisor.md). **Layout ADRs:** [ADR-0019](../../../docs/adr/ADR-0019-agentworker-internal-layout.md), [ADR-0020](../../../docs/adr/ADR-0020-realtime-sse-layout.md) (SSE publish port).

## Files

| File | Role |
|------|------|
| `supervisor.go` | `Supervisor`, `New`, `Start`, `Reload`, `Drain`, `CancelCurrentRun`, `ProbeRunner` |
| `apply.go` | `applySettings` pipeline and reload branches |
| `instance.go` | Worker instance spawn/stop, `instanceSnapshot` |
| `runner_build.go` | Execute/verify runner build, orphan sweep, scheduling hint probe |
| `sse.go` | Cycle change and run-progress SSE adapters |
| `paths.go` | Repo root and report directory guards |
| `test_hooks.go` | Exported test helpers for black-box tests |
| `policy/` | Pure idle gating and material-change comparison |

## Wiring

`cmd/taskapi/run_agentworker.go` calls `agentworker.New` + `Start` after queue/reconcile boot, passing `handler.NewSSEHubWith(...)` as `realtime.Publisher`. The handler receives the supervisor as `handler.AgentWorkerControl`.

Metrics registration stays in [`../agent_worker_metrics.go`](../agent_worker_metrics.go) and is injected at `New`.
