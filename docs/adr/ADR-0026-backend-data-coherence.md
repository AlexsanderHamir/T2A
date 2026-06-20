# ADR-0026: Backend Data Coherence (Read Policy + Enriched SSE)

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Engineering

## Context

ADR-0025 centralized frontend query and mutation policy. The backend still had scattered bootstrap limits, hand-rolled commit-then-notify in each handler, and an enrichment gap: task PATCH/retry published full `domain.Task` in SSE `data`, while checklist and gate mutations published id-only `task_updated` hints — forcing the SPA to refetch on paths where `decideSyncFrame` already supports inline patch.

## Decision

### Read policy

Centralize bootstrap (and future shell) limits in [`pkgs/tasks/handler/readpolicy/readpolicy.go`](../../pkgs/tasks/handler/readpolicy/readpolicy.go). [`handler_bootstrap.go`](../../pkgs/tasks/handler/handler_bootstrap.go) imports these constants instead of local magic numbers.

| Constant | Value | SPA mirror |
|----------|-------|------------|
| `BootstrapListLimit` | 20 | `TASK_LIST_PAGE_SIZE` |
| `BootstrapProjectsLimit` | 100 | `useProjects({ limit: 100 })` |
| `BootstrapDraftsLimit` | 50 | create-flow drafts |

### Write / publish policy

Add [`handler_writepolicy.go`](../../pkgs/tasks/handler/handler_writepolicy.go) with `notifyTaskUpdatedEnriched` and pure classification in [`writepolicy/publish_policy.go`](../../pkgs/tasks/handler/writepolicy/publish_policy.go).

**Invariants:**

| ID | Rule |
|----|------|
| S1 | `hub.Publish` for mutation hints never runs before store returns nil |
| S2 | Enriched `task_updated` uses post-commit `store.Get` |
| S3 | Hint-only events remain id-only: `task_deleted`, `task_gate_changed`, `task_dependency_changed`, `project_*`, `settings_changed` |
| S4 | Harness/worker cycle publishes unchanged |

**SSE publish table:**

| Event | Payload | When |
|-------|---------|------|
| `task_created`, `task_updated` | Enriched `domain.Task` in `data` | Task row changed (CRUD, checklist, gate action, retry) |
| `task_deleted` | Hint only | Delete succeeded |
| `task_gate_changed`, `task_dependency_changed` | Hint only | Sidecar resource changed |
| `task_cycle_changed` | Enriched cycle (existing) | Unchanged |

Checklist and gate handlers now use enriched `task_updated` after successful mutations.

### Bootstrap list builder

Extract `buildListResponse` shared by `GET /tasks` and bootstrap tasks envelope so wire shapes stay identical.

### Task detail shell (deferred)

`GET /v1/tasks/{id}/shell` is **not shipped** in this ADR. Phase 2 trigger: frontend RUM p95 `navigation.task_detail.time_to_interactive_ms` > 300ms on LAN after ADR-0025 + enriched SSE. With parallel checklist fetch and enriched SSE live on the client, the aggregate route is deferred until metrics prove it necessary.

## Consequences

### Positive

- Bootstrap limits documented in one module cross-linked from `docs/api.md`
- Checklist/gate SSE matches CRUD enrichment; fewer SPA refetches
- Shared list builder reduces bootstrap/list drift risk
- Pure `readpolicy` / `writepolicy` subpackages enforceable in CI

### Negative / Trade-offs

- Enriched publish adds one `Get` after checklist mutations (same cost as PATCH today)
- Shell API remains optional follow-up if RUM misses target

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Redis read-through cache | Out of scope; Postgres authoritative |
| GraphQL aggregate | Overkill for two-field shell |
| Hint-only checklist SSE | Leaves ADR-0025 sync path doing extra GETs |

## Related

- [ADR-0025](ADR-0025-frontend-data-coherence.md) — client query/mutation policy
- [docs/domain/sse-hub.md](../domain/sse-hub.md) — hub behavior and enrichment
- [docs/api.md](../api.md) — bootstrap limits reference `readpolicy`
