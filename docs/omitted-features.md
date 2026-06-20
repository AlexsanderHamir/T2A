# Omitted features (launch registry)

Features that **exist in the codebase** but are **hidden or fixed for a specific launch**. Use this file when you need to know what operators and contributors should not expect in the UI yet, without deleting backend routes, stores, or tests.

**Code switch:** `web/src/launch/omittedFeatures.ts` — UI reads `isUiFeatureOmitted(...)`. Keep the doc and that module in sync when adding or restoring a feature.

**Not the same as:**

- [docs/api.md](./api.md) — full HTTP contract (omitted UI does not remove API routes).
- [docs/adr/](./adr/) — permanent architecture decisions.
- Deleted or deprecated behavior — omitted features stay implemented; they are just not exposed.

---

## How to use this file

| Role | Action |
| --- | --- |
| **Product / launch** | Add a row when a feature ships in code but not in the operator UI for a target release. |
| **Web** | Gate UI with `isUiFeatureOmitted` from `web/src/launch/omittedFeatures.ts`; link the gate in the table below. |
| **Backend** | Usually no change — APIs and persistence stay available for tests, migrations, and later UI. |
| **Restore** | Set the flag to `false`, remove UI gates, update status to **Restored**, and note the release in the changelog row. |

---

## Active omissions

### Projects (UI + task assignment)

| Field | Value |
| --- | --- |
| **Status** | Omitted (initial launch) |
| **Since** | 2026-06-20 |
| **Target restore** | TBD — when multi-project workflows are launch-ready |

**Operator-visible behavior**

- No **Projects** item in the primary nav.
- `/projects` and nested project routes redirect to `/`.
- Task list: no **Project** column and no project filter.
- Create / edit task modal: no project picker and no project context attachment UI.
- New and edited tasks still persist with the **default project** (`DEFAULT_PROJECT_ID` in `web/src/types/project.ts`).

**Still implemented (intentionally not deleted)**

- REST: `GET/POST /projects`, `GET/PATCH/DELETE /projects/{id}`, project context routes — see [api.md](./api.md).
- Postgres seed of the built-in default project (`pkgs/tasks/postgres/postgres.go`).
- `web/src/projects/` pages, hooks, and tests (reachable in tests; not linked from launch UI).
- `project_id` on tasks in the data model — [data-model.md](./data-model.md).

**UI gates**

| Surface | File |
| --- | --- |
| Nav + route redirect | `web/src/app/App.tsx` |
| Create/edit modal assignment | `web/src/tasks/pages/TaskCreateModalsLayer.tsx` |
| List filter + projects query | `web/src/tasks/pages/TaskHome.tsx` |
| Project column | `web/src/tasks/components/task-list/section/TaskListSection.tsx`, `.../table/TaskListDataTable.tsx` |

**Restore checklist**

- [ ] Set `projects: false` in `web/src/launch/omittedFeatures.ts`.
- [ ] Smoke-test nav, `/projects`, create modal picker, list filter/column.
- [ ] Move this section to **Restored** below with the release name/date.

---

### Tags & dependencies (create/edit modal)

| Field | Value |
| --- | --- |
| **Status** | Omitted (initial launch) |
| **Since** | 2026-06-20 |
| **Target restore** | TBD — when tag/milestone/dependency editing is launch-ready |

**Operator-visible behavior**

- Create / edit task modal **More options**: no **Tags & dependencies** fieldset (tags, milestone, depends-on picker).
- Collapsed **More options** summary no longer mentions tags or dependencies (shows agent only when schedule is also omitted).
- New tasks still submit with empty tags, no milestone, and no `depends_on` edges unless set via API.

**Still implemented (intentionally not deleted)**

- Task fields `tags`, `milestone`, and dependency edges in the data model — [data-model.md](./data-model.md).
- REST dependency routes and task PATCH fields — [api.md](./api.md).
- Task detail scheduling UI (if present) and store logic unchanged.

**UI gates**

| Surface | File |
| --- | --- |
| Modal fieldset + summary hint | `web/src/tasks/components/task-create-modal/TaskCreateModal.tsx` |
| Summary line copy | `web/src/tasks/components/task-create-modal/advancedSummaryLine.ts` |

**Restore checklist**

- [ ] Set `tagsAndDependencies: false` in `web/src/launch/omittedFeatures.ts`.
- [ ] Smoke-test create and edit modals: tags, milestone, depends-on picker.
- [ ] Move this section to **Restored** below with the release name/date.

---

### Schedule for (create/edit modal)

| Field | Value |
| --- | --- |
| **Status** | Omitted (initial launch) |
| **Since** | 2026-06-20 |
| **Target restore** | TBD — when deferred pickup scheduling is launch-ready |

**Operator-visible behavior**

- Create / edit task modal **More options**: no **Schedule for** fieldset (`SchedulePicker` / pickup schedule field).
- Collapsed **More options** summary no longer mentions schedule (shows agent only when all secondary fields are omitted).
- New tasks omit `pickup_not_before` on create — worker picks up when free (same as “Picks up immediately”).

**Still implemented (intentionally not deleted)**

- `pickup_not_before` on tasks and scheduling predicates — [data-model.md](./data-model.md), [docs/domain/task-scheduling.md](./domain/task-scheduling.md).
- Task detail schedule UI and bulk reschedule on the list unchanged.
- REST PATCH/POST still accept `pickup_not_before` — [api.md](./api.md).

**UI gates**

| Surface | File |
| --- | --- |
| Modal schedule fieldset | `web/src/tasks/components/task-create-modal/TaskCreateModal.tsx` |
| Summary line copy | `web/src/tasks/components/task-create-modal/advancedSummaryLine.ts` |

**Restore checklist**

- [ ] Set `schedule: false` in `web/src/launch/omittedFeatures.ts`.
- [ ] Smoke-test create and edit modals: schedule picker and deferred pickup copy.
- [ ] Move this section to **Restored** below with the release name/date.

---

## Restored (history)

_None yet._
