# ADR-0025: Frontend Data Coherence (Query Policy + Mutation Guard)

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Engineering

## Context

After ADR-0022 (SSE sync) and ADR-0024 (create flow slice), client-side data policy remained fragmented: stale times scattered across hooks, checklist optimistic writes without `mutationGuard`, and [`useTasksApp`](../../web/src/tasks/hooks/useTasksApp.ts) flat return causing wide re-renders during draft autosave.

## Decision

### Query policy

Centralize tiers in [`web/src/tasks/queryPolicy.ts`](../../web/src/tasks/queryPolicy.ts):

| Tier | staleTime | Keys |
|------|-----------|------|
| Shell | 5 min | settings, projects, automations |
| List | 60s | task list pages, stats |
| Detail | 30s | task detail, checklist, cycles, commits |
| Prefetch | 30s | hover detail prefetch |
| Default | 15s | fallback |

Document read order in [`docs/web.md`](../web.md).

### Mutation coherence

Introduce [`web/src/tasks/mutations/`](../../web/src/tasks/mutations/) — shared guarded optimistic helpers. Extend `beginTaskMutation` / `endTaskMutation` from [`sync/mutationGuard`](../../web/src/tasks/sync/mutationGuard.ts) to checklist mutations via [`web/src/tasks/checklist/`](../../web/src/tasks/checklist/) slice.

**Invariants:**

| ID | Rule |
|----|------|
| M1 | Task-scoped mutation bumps guard before optimistic write |
| M2 | Bulk N-task PATCH invalidates list once — no per-task guard |
| M3 | Create-flow draft refs remain separate (ADR-0024 I2–I5) |

### Render isolation

[`TasksAppProvider`](../../web/src/tasks/app/TasksAppProvider.tsx) + selector hooks (`useTasksAppList`, `useTasksAppModals`, `useTasksAppMeta`) narrow subscriptions.

### Session persist (optional)

`@tanstack/react-query-persist-client` for shell keys only; disabled when `VITE_QUERY_PERSIST=0`. Bust on SSE `resync`.

## Consequences

### Positive

- One module answers staleTime questions
- Checklist edits survive SSE echo during agent runs
- Detail navigation latency reduced via parallel fetch + prefetch
- List rows decoupled from draft autosave ticks

### Negative / Trade-offs

- TasksAppProvider migration is incremental; `app` prop shims remain briefly
- Persist adds dependency; kill-switch required for tests

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Normalized entity cache | High migration cost; invalidate model works with enrichment |
| GraphQL | Out of scope |
| React Query persist for all keys | Agent-run volatile data must not persist |

## Related

- [ADR-0022](ADR-0022-task-sync-policy.md), [ADR-0024](ADR-0024-task-create-flow-slice.md)
- [docs/web.md](../web.md)
