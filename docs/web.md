# Web SPA

Vite + React client under `web/`. All `fetch` calls live in `web/src/api/`; responses are parsed through typed parsers before use.

| | |
| --- | --- |
| **Applies to** | `web/` SPA routes, data layer, and task UI |
| **Audience** | Frontend contributors and agents on web-only or full-stack slices |
| **Prerequisite** | [architecture.md](./architecture.md) for API/SSE context; [api.md](./api.md) for contracts |

## In this article

- [Routes](#routes)
- [Cold start](#cold-start)
- [Task sync (SSE cache coherence)](#task-sync-sse-cache-coherence)
- [Task create flow](#task-create-flow)
- [Query policy](#query-policy)
- [Task detail — execution cycles](#task-detail--execution-cycles)
- [See also](#see-also)

## Routes

| Path | Module | Notes |
| --- | --- | --- |
| `/` | `web/src/tasks/` | Task home list |
| `/templates` | `web/src/tasks/` | Saved task templates (search, batch instantiate) |
| `/drafts` | `web/src/tasks/` | Saved create-task drafts |
| `/projects` | `web/src/projects/` | Project list |
| `/projects/:id` | `web/src/projects/` | Project detail |
| `/settings` | `web/src/settings/` | App settings |
| `/tasks/:id` | `web/src/tasks/pages/` | Task detail |

Primary nav links: Tasks, Templates, Drafts, Projects (Settings is header gear).

## Cold start

`web/src/app/hooks/useBootstrap.ts` seeds TanStack Query from `GET /v1/bootstrap` (settings, root task list, stats, projects, draft head). Per-page hooks fall back to individual GETs when bootstrap is absent.

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
3. `composePayload.ts`, `validateCreateForm.ts`, `draftPayload.ts`, `buildCreateMutationInput.ts` — shared compose payload, validation, and wire shapes
4. `mapCreateFlowViewModel.ts` — flat public return shape for `useTasksApp`
5. `hooks/useTaskCreateFlow.ts` — composer; shim at `web/src/tasks/hooks/useTaskCreateFlow.ts`

Modal UI stays in `web/src/tasks/components/task-create-modal/` for V1. **`composeTarget`** (`task` | `template`) and **`composeOperation`** (`create` | `edit`) drive one modal for task create/edit and template save/edit. Templates list and batch create: `web/src/tasks/pages/TaskTemplatesPage.tsx` (`GET /task-templates`, `POST /task-templates/instantiate`). API client: `web/src/api/taskTemplates.ts`. Race contracts: `useTasksApp.test.tsx`.

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

## See also

- [guide.md](./guide.md) — documentation layers and learning paths
- [README.md](./README.md) — doc index by topic
- [agent-map.md](./agent-map.md) — web code paths
- [CONTRIBUTING.md](../CONTRIBUTING.md) — setup and PR checklist
- [domain/sse-hub.md](./domain/sse-hub.md) — SSE event catalog and operator tuning
