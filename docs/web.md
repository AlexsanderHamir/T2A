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

## Task detail — execution cycles

Expanded cycle rows in `TaskCyclesPanel` load `GET /tasks/{id}/cycles/{cycleId}/verdicts`. When the worker indexed git commits for the cycle, the panel shows a repo → branch breadcrumb and commit rows (`git_context`, `commits[]`) above the per-criterion verdict list. Parsers: `web/src/api/parseTaskApiCycles.ts`; types: `web/src/types/cycle.ts`. See [domain/cycle-commits.md](./domain/cycle-commits.md).
