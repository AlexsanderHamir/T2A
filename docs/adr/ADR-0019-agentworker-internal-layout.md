# ADR-0019: Agent Worker Supervisor Internal Layout

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Engineering

## Context

The in-process agent worker supervisor lives in [`cmd/taskapi/run_agentworker.go`](../../cmd/taskapi/run_agentworker.go) (~870 lines). It mixes lifecycle (Start/Reload/Drain), settings apply pipeline, runner probe/build, orphan sweep, SSE adapters, path guards, and pure idle/material-change policy. Tests live in `cmd/taskapi` beside the binary entrypoint.

The file exceeds the repo red bar. Contributors must grep one file to change hot reload or verify demotion. Policy tests require SQLite and a full supervisor rig.

[`docs/domain/agent-supervisor.md`](../domain/agent-supervisor.md) already documents logical domains; the code layout does not match.

## Decision

Extract supervisor implementation into **`internal/taskapi/agentworker/`** with at most one nested subpackage:

| Package | Responsibility |
|---------|----------------|
| `internal/taskapi/agentworker/policy` | Pure idle gating, scheduling hint, material-change comparison, verify-runner status labels |
| `internal/taskapi/agentworker` | `Supervisor`, apply pipeline, instance lifecycle, runner build, SSE adapters, path guards |

**Public API:** `agentworker.Supervisor`, `New`, `Start`, `Reload`, `Drain`, `CancelCurrentRun`, `ProbeRunner`. Satisfies [`handler.AgentWorkerControl`](../../pkgs/tasks/handler/handler.go) via implicit interface compliance. `cmd/taskapi` injects the concrete supervisor into `internal/taskapi.NewHTTPHandler`.

**Stays in `cmd/taskapi`:** `startReadyTaskAgents` — queue cap, pickup wake, reconcile loop, then `agentworker.New` + `Start`. Reconcile always runs; the supervisor only gates the worker goroutine.

**Stays in `internal/taskapi` (sibling):** [`agent_worker_metrics.go`](../../internal/taskapi/agent_worker_metrics.go) — Prometheus registration injected at `New`; not supervisor policy.

### Dependency rules

```
cmd/taskapi → internal/taskapi/agentworker
agentworker → agentworker/policy, store, worker, registry, handler (SSEHub), internal/taskapi (metrics)
policy      → store (AppSettings fields only)
```

**Forbidden:** `policy` importing `agentworker`. **Forbidden:** `handler` importing `internal/taskapi/agentworker`.

### Migration

Track A: **move-only** — behavior, SSE payloads, idle reasons, and metrics unchanged. Five commits (+ push to `main`) per cycle:

| Cycle | Commit |
|-------|--------|
| 0 | ADR-0019 |
| 1 | Extract `policy` |
| 2 | Move supervisor body |
| 3 | Relocate tests + funclog paths |
| 4 | Domain doc path updates |

### Non-goals (Track A)

- No `pkgs/agents/supervisor` public package
- No FSM for `applySettings`
- No SSE hub interface ports (Track B)
- No moving queue/reconcile boot out of `cmd`

## Consequences

### Positive

- `cmd/taskapi` returns to wiring-only for supervisor logic
- Policy unit tests without SQLite
- File-size bar achievable per file
- Domain doc maps to package layout

### Negative / Trade-offs

- DTO mapping at policy boundary (`InstanceSnapshot`)
- `funclogmeasure` allowlist path updates
- Temporary churn during phased migration

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| `pkgs/agents/supervisor` | Taskapi-specific; no second importer today |
| Single file split in `cmd` | Go `internal/` rule not leveraged; still in main |
| Merge metrics into agentworker | Metrics are startup registration, not lifecycle |
| Move queue/reconcile with supervisor | Process boot ≠ worker gating; reconcile must run when paused |

## Related

- [ADR-0005](ADR-0005-extract-agent-harness.md) — harness extraction from worker
- [ADR-0017](ADR-0017-harness-internal-domains.md) — pattern for internal domain split
- [docs/domain/agent-supervisor.md](../domain/agent-supervisor.md) — behavioral reference
