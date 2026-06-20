# Web SPA

Vite + React client under `web/`. All `fetch` calls live in `web/src/api/`; responses are parsed through typed parsers before use.

## Routes

| Path | Module | Notes |
| --- | --- | --- |
| `/` | `web/src/tasks/` | Task home list |
| `/drafts` | `web/src/tasks/` | Saved create-task drafts |
| `/projects` | `web/src/projects/` | Project list |
| `/projects/:id` | `web/src/projects/` | Project detail |
| `/automations` | `web/src/automations/` | Global prompt-automation library CRUD |
| `/settings` | `web/src/settings/` | App settings |
| `/tasks/:id` | `web/src/tasks/pages/` | Task detail |

Primary nav links: Tasks, Drafts, Projects, Automations (Settings is header gear).

## Cold start

`web/src/app/hooks/useBootstrap.ts` seeds TanStack Query from `GET /v1/bootstrap` (settings, root task list, stats, projects, automations catalog, draft head). Per-page hooks fall back to individual GETs when bootstrap is absent.

## Prompt automations (UI)

- **Library:** `AutomationsPage` — list + modal create/edit; archives via `DELETE /automations/{id}`.
- **Create task:** `AutomationPicker` in the create modal (Behaviors section) — browse modal with search, segmented **Yes / Omit / No** per row. Omit removes the selection from `automation_selections`; only yes/no are sent to `POST /tasks`.
- **API:** `web/src/api/automations.ts`, types in `web/src/types/automation.ts`.
- **Query keys:** `web/src/automations/queryKeys.ts` (`automationQueryKeys.list(includeArchived, limit)`).

Draft autosave persists `automation_selections` in `TaskDraftPayload` via `draftAutosaveSignature`.

Task detail edit UI for automations is not in V1; PATCH `/tasks/{id}` accepts `automation_selections` for follow-up work.

See [ADR-0013](./adr/ADR-0013-prompt-automations.md).

## Task sync (SSE cache coherence)

Live task UI cache policy lives in [`web/src/tasks/sync/`](../../web/src/tasks/sync/). Read order:

1. [ADR-0022](./adr/ADR-0022-task-sync-policy.md) — Decide vs Apply boundaries
2. `decideSyncFrame.ts` — per-frame schedule, suppression, enrichment effects
3. `decideFlushBatch.ts` — debounced invalidation targets
4. `taskSyncCoordinator.ts` — pending state + debounce wiring consumed by `useTaskEventStream`

Wire decode stays in `web/src/tasks/task-query/sseInvalidate.ts`. Event catalog and operator tuning: [domain/sse-hub.md](./domain/sse-hub.md).

## Task create flow

Create-task policy and hook composition live in [`web/src/tasks/create/`](../web/src/tasks/create/). Read order:

1. [ADR-0024](./adr/ADR-0024-task-create-flow-slice.md) — Decide vs Apply boundaries, invariants I1–I7
2. `decideCreateEntry.ts` — `openCreateModal` routing (loading / error / drafts / fresh)
3. `validateCreateForm.ts`, `draftPayload.ts`, `buildCreateMutationInput.ts` — pure validation and wire payloads
4. `mapCreateFlowViewModel.ts` — flat public return shape for `useTasksApp`
5. `hooks/useTaskCreateFlow.ts` — composer; shim at `web/src/tasks/hooks/useTaskCreateFlow.ts`

Modal UI stays in `web/src/tasks/components/task-create-modal/` for V1. Race contracts: `useTasksApp.test.tsx`.

## Query policy

TanStack Query staleTime tiers live in [`web/src/tasks/queryPolicy.ts`](../web/src/tasks/queryPolicy.ts). Read order:

1. [ADR-0025](./adr/ADR-0025-frontend-data-coherence.md) — query tiers, mutation guard M1–M3, render isolation
2. `queryPolicy.ts` — `QUERY_POLICY` constants consumed by `queryClient`, list hooks, prefetch
3. [`tasks/mutations/`](../web/src/tasks/mutations/) — guarded optimistic task writes
4. [`tasks/checklist/`](../web/src/tasks/checklist/) — detail checklist mutations with guard
5. [`tasks/app/TasksAppProvider.tsx`](../web/src/tasks/app/TasksAppProvider.tsx) — narrow selector hooks

## Task detail — execution cycles

Expanded cycle rows in `TaskCyclesPanel` load `GET /tasks/{id}/cycles/{cycleId}/verdicts`. When the worker indexed git commits for the cycle, the panel shows a repo → branch breadcrumb and commit rows (`git_context`, `commits[]`) with **status badges** (`eligible`, `observed`, …) above the per-criterion verdict list.

The task detail page also loads **`GET /tasks/{id}/commits`** via `TaskCommitsPanel` / `useTaskCommits` — task-wide commit history deduped by SHA, refetched on `task_cycle_changed` SSE. Clicking a commit row navigates to **`/tasks/{id}/commits/{sha}`** (`TaskCommitDiffPage`), which loads **`GET /repo/diff?sha=`** with GitHub-style summary stats, syntax-highlighted hunks (refractor + `react-diff-view`), unified/split toggle, file navigator, and collapsible large files. Parsers: `web/src/api/parseTaskApiCycles.ts`; types: `web/src/types/cycle.ts`. See [domain/cycle-commits.md](./domain/cycle-commits.md).
