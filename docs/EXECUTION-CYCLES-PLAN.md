# Execution cycles — implementation plan

This document is the **agreed working breakdown** for promoting the **diagnose → execute → verify → persist** loop from prose in [`moat.md`](../moat.md) to a first-class primitive in the data model, while keeping `task_events` as the flat append-only audit witness.

**Design rationale and tradeoffs:** kept in this doc inline (no separate ADR yet); promote to a contract doc (`docs/EXECUTION-CYCLES.md`) in **Stage 6**.

## Rules of engagement

1. **One stage per PR / commit.** Stages are sized so each leaves the repo **buildable, tested, linted, and shippable**.
2. **Verification gate per stage.** A stage is not "done" until its checklist is green AND `./scripts/check.ps1` (or the documented Go-only fast path) passes locally.
3. **Commit + push at end of stage.** Conventional commit message, one logical concern, push to current branch (`main` unless redirected). See [`AGENTS.md`](../AGENTS.md) "Commands to run before you finish".
4. **STOP and ask permission between stages.** No silent rollover; the user explicitly OKs each next stage.
5. **TDD default** per [`AGENTS.md`](../AGENTS.md): failing test first when adding behavior, then make it green.
6. **No scope creep.** If a stage discovers extra work, append it to a `### Notes / followups` block at the bottom of this file rather than expanding the active stage.

## Reference points

- **Closest analog in repo:** the **checklist** slice (`pkgs/tasks/store/store_checklist*.go`, `pkgs/tasks/handler/handler_checklist.go`, `web/src/tasks/components/task-detail/checklist/`). Same shape: definitions table + state table + dual-write to `task_events` + REST + web UI + contract tests.
- **Vertical slice convention:** [`docs/EXTENSIBILITY.md`](./EXTENSIBILITY.md) (`domain → store → handler → web`).
- **Engineering bar:** `.cursor/rules/BACKEND_AUTOMATION/backend-engineering-bar.mdc`, `.cursor/rules/UI_AUTOMATION/web-ui-engineering-bar.mdc`.
- **Test recipes:** `.cursor/rules/BACKEND_AUTOMATION/go-testing-recipes.mdc`, `.cursor/rules/UI_AUTOMATION/testing-recipes.mdc`.

## What we're building (one-screen recap)

Two new tables, owned only by the store layer through one chokepoint API:

- **`task_cycles`** — one row per execution attempt for a task (`id`, `task_id` FK, `attempt_seq`, `status`, `started_at` / `ended_at`, `triggered_by`, `parent_cycle_id?`, `meta_json`).
- **`task_cycle_phases`** — one row per phase entry within a cycle (`id`, `cycle_id` FK, `phase` enum, `phase_seq`, `status`, `started_at` / `ended_at`, `summary?`, `details_json`, `event_seq?`).

Every cycle/phase write also appends a mirror row to `task_events` **in the same SQL transaction**, so `GET /tasks/{id}/events` keeps showing a complete timeline and existing observability is unchanged.

## Stages

Each stage's "Exit criteria" is the gate. Verification commands are listed once at the bottom under [Common verification](#common-verification).

---

### Stage 0 — Plan landed (this doc)

- [x] `docs/EXECUTION-CYCLES-PLAN.md` written.
- [x] Linked from `docs/AGENTIC-LAYER-PLAN.md` and `docs/README.md`.
- [x] Commit + push.

**Commit:** `docs: add execution-cycles implementation plan`

**STOP — ask permission to begin Stage 1.**

---

### Stage 1 — Domain layer (types + state machine, no DB, no I/O)

**Scope (touch only `pkgs/tasks/domain/`):**

- [ ] Add `Phase` enum (`diagnose`, `execute`, `verify`, `persist`) in `enums.go`.
- [ ] Add `CycleStatus` enum (`running`, `succeeded`, `failed`, `aborted`).
- [ ] Add `PhaseStatus` enum (`running`, `succeeded`, `failed`, `skipped`).
- [ ] Add 7 new `EventType` constants: `cycle_started`, `cycle_completed`, `cycle_failed`, `phase_started`, `phase_completed`, `phase_failed`, `phase_skipped`. (Constants only; nothing emits them yet.)
- [ ] Add `TaskCycle` and `TaskCyclePhase` GORM model structs in `models.go`. Tags written but no `AutoMigrate` registration yet.
- [ ] Add new file `cycle_state.go` with `func ValidPhaseTransition(prev, next Phase) bool` and a sibling `func TerminalCycleStatus(s CycleStatus) bool`.
- [ ] Add `cycle_state_test.go` covering: valid forward transitions, valid `verify → execute` re-entry, all invalid transitions rejected, terminal-status helper truth table.
- [ ] Update `pkgs/tasks/domain/doc.go` if needed.

**Exit criteria:**

- `go vet ./pkgs/tasks/domain/...` clean.
- `go test ./pkgs/tasks/domain/... -count=1` passes including new tests.
- `funclogmeasure -enforce` green on touched files (per `docs/OBSERVABILITY.md`).
- No changes outside `pkgs/tasks/domain/`.

**Commit:** `domain: add execution cycle types, state machine, and event type constants`

**STOP — ask permission to begin Stage 2.**

---

### Stage 2 — Schema migration + store CRUD (no dual-write yet)

**Scope:**

- [ ] Register `domain.TaskCycle` and `domain.TaskCyclePhase` in `pkgs/tasks/postgres/postgres.go::Migrate` `AutoMigrate` call.
- [ ] Decide partial-unique-index strategy for "one running cycle per task" + "one running phase per cycle":
  - **Default chosen:** in-TX `SELECT ... LIMIT 1` guard inside store (portable across Postgres + SQLite); document the choice in this file's followups section.
  - If we later want a real partial index on Postgres, add as a post-AutoMigrate raw SQL hook.
- [ ] New file `pkgs/tasks/store/store_cycles.go` with: `StartCycle`, `TerminateCycle`, `GetCycle`, `ListCyclesForTask`.
- [ ] New file `pkgs/tasks/store/store_cycle_phases.go` with: `StartPhase`, `CompletePhase`, `ListPhasesForCycle`.
- [ ] Validate inputs at the store boundary (status enum, phase enum, transition validity via `domain.ValidPhaseTransition`); map invalid input to `domain.ErrInvalidInput`.
- [ ] New file `pkgs/tasks/store/store_cycles_test.go` (table-driven) — happy path + all invariant violations.

**Out of scope for this stage:** `task_events` mirror writes. Pure cycle/phase state writes only — keeps the diff small and the patterns pure.

**Exit criteria:**

- `go test ./pkgs/tasks/store/... -count=1` passes.
- `go test ./pkgs/tasks/postgres/... -count=1` passes (AutoMigrate still works on SQLite).
- `internal/tasktestdb` test fixture still opens cleanly with the new tables.
- `funclogmeasure -enforce` clean.

**Commit:** `store: add task_cycles + task_cycle_phases tables and CRUD operations`

**STOP — ask permission to begin Stage 3.**

---

### Stage 3 — Dual-write mirror to `task_events`

**Scope:**

- [ ] Inside each public store function from Stage 2, append the corresponding mirror `task_events` row in the **same `gorm.DB` transaction**.
- [ ] Capture the assigned `task_events.seq` and write it back into the cycle/phase row (`event_seq` column) so the audit pointer is one-shot.
- [ ] Add `pkgs/tasks/store/store_cycles_dualwrite_test.go` that pins the invariant: for **every** public mutating cycle/phase entry point, an audit row exists with the matching event type and `data_json` payload, OR the cycle/phase write was rolled back. (Table-driven across all entry points; failure forces future contributors to update both sides.)
- [ ] Confirm `EventTypeAcceptsUserResponse` (in `pkgs/tasks/domain/event_user_response.go`) does **not** include the new types — they are observational, not interactive.

**Exit criteria:**

- `go test ./pkgs/tasks/store/... -count=1` passes including the new dual-write invariant suite.
- `go test ./pkgs/tasks/handler/... -count=1` still passes (no API change yet).
- `funclogmeasure -enforce` clean.

**Commit:** `store: mirror cycle and phase transitions into task_events in the same transaction`

**STOP — ask permission to begin Stage 4.**

---

### Stage 4 — HTTP handler routes

**Scope (touch only `pkgs/tasks/handler/` + `internal/taskapi/` for mux registration):**

- [ ] New file `pkgs/tasks/handler/handler_cycles.go` exposing six routes:
  - `POST /tasks/{id}/cycles` — `Idempotency-Key` honored; body `{triggered_by?, meta?, parent_cycle_id?}`.
  - `GET  /tasks/{id}/cycles` — keyset paging matching `/events` conventions.
  - `GET  /tasks/{id}/cycles/{cycleId}` — embedded `phases[]`.
  - `POST /tasks/{id}/cycles/{cycleId}/phases` — body `{phase, summary?}`.
  - `PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}` — body `{status, summary?, details?}`; state machine validates.
  - `PATCH /tasks/{id}/cycles/{cycleId}` — body `{status, reason?}`.
- [ ] JSON DTOs colocated; reject unknown fields and trailing data (existing handler convention).
- [ ] `X-Actor` plumbed through to store via existing `actorFromRequest` helper.
- [ ] All abuse guards: 128-byte path segment caps, body field length caps consistent with current handler bar.
- [ ] Mux registration in the same place as other resource families.
- [ ] Tests:
  - `handler_http_cycles_test.go` — happy paths for all six routes (whitebox where helpers needed).
  - `handler_http_cycles_contract_test.go` — pin every documented 400 string, every JSON shape, response key set, content-type, status code; mirror Sessions 4–8 pattern from `.agent/backend-improvement-agent.log`.

**Exit criteria:**

- `go vet ./...` clean.
- `go test ./... -count=1` passes.
- `funclogmeasure -enforce` clean.
- New routes appear in `pkgs/tasks/handler/README.md` file-map.

**Commit:** `handler: add task execution cycle and phase REST routes with contract tests`

**STOP — ask permission to begin Stage 5.**

---

### Stage 5 — SSE trigger surface

**Scope:**

- [ ] Add `task_cycle_changed` SSE event type with payload `{type: "task_cycle_changed", id: <task_id>, cycle_id: <cycle_id>}`.
- [ ] Call `notifyChange` (or the cycle-specific equivalent) from each cycle-mutating handler write path.
- [ ] Extend `pkgs/tasks/handler/sse_trigger_surface_test.go` with subtests covering every new mutating route → assert exact published `{type, id, cycle_id}` set.
- [ ] Update `docs/API-SSE.md` trigger table.

**Exit criteria:**

- All SSE trigger surface tests pass with new subtests included.
- `docs/API-SSE.md` and the test stay in sync (docs PR contract).

**Commit:** `handler: publish task_cycle_changed on cycle and phase mutations`

**STOP — ask permission to begin Stage 6.**

---

### Stage 6 — Backend docs + contract pinning

**Scope (docs-only, no code changes):**

- [ ] New `docs/EXECUTION-CYCLES.md` — design rationale, schema diagram, dual-write invariant, state machine diagram, "where reads go" table, concurrency rules, what's intentionally out.
- [ ] `docs/API-HTTP.md` — add cycle routes to handler-routes table; add new 400 string subsection; cross-reference SSE trigger surface.
- [ ] `docs/DESIGN.md` — add a "Cycles vs flat audit log" note to **Limitations** (single source of truth, dual-write rationale, no consolidation).
- [ ] `docs/AGENT-QUEUE.md` — short note that workers will use cycles; link to plan.
- [ ] `docs/AGENTIC-LAYER-PLAN.md` — strike-through or check the V1 line item that motivated this work; link to `EXECUTION-CYCLES.md`.
- [ ] `AGENTS.md` repo-map — new row for `pkgs/tasks/handler/handler_cycles.go` + new domain types.
- [ ] `docs/README.md` index — new row for `EXECUTION-CYCLES.md`.

**Exit criteria:**

- `./scripts/check.ps1` with `CHECK_SKIP_WEB=1` (docs-only fast path).
- All cross-links resolve (manual scan).

**Commit:** `docs: document task execution cycles primitive and update API + design references`

**STOP — ask permission to begin Stage 7.**

---

### Stage 7 — Web data layer

**Scope (touch only `web/src/api/`, `web/src/types/`, `web/src/tasks/task-query/`, `web/src/tasks/hooks/`):**

- [ ] `web/src/types/cycle.ts` — TS types for `TaskCycle`, `TaskCyclePhase`, status/phase enums.
- [ ] Re-export from `web/src/types/index.ts` barrel (`@/types`).
- [ ] `web/src/api/cycles.ts` — `listTaskCycles`, `getTaskCycle`, `startTaskCycle`, `terminateTaskCycle`, `startTaskCyclePhase`, `patchTaskCyclePhase` (mirror existing `tasks.ts` conventions).
- [ ] Extend `web/src/api/parseTaskApi.ts` with shape parsers for the new types; tests in `parseTaskApi.test.ts`.
- [ ] Re-export from `web/src/api/index.ts`.
- [ ] `web/src/tasks/task-query/index.ts` (or `taskQueryKeys.ts`) — add `taskQueryKeys.cycles(taskId)` and `taskQueryKeys.cycle(taskId, cycleId)`.
- [ ] `web/src/tasks/task-query/sseInvalidate.ts` — handle `task_cycle_changed` to invalidate **only** the affected task's cycle caches (not the whole task tree); test pins the granularity.
- [ ] `web/src/tasks/hooks/useTaskCycles.ts` + colocated test using `MockEventSource` + mocked fetch (existing recipe).

**Exit criteria:**

- `cd web && npm test -- --run` passes.
- `npm run lint` clean.
- `npm run build` clean.

**Commit:** `web: add task cycles API client, query keys, SSE invalidation, and useTaskCycles hook`

**STOP — ask permission to begin Stage 8.**

---

### Stage 8 — Web UI panel (optional MVP cut — confirm before starting)

**Default decision:** ship this only if the user explicitly says so. The mirror `task_events` already render via `TaskUpdatesTimeline`, so the existing UI already shows cycle activity once the backend lands. Skipping this stage is a **valid MVP**.

**If approved, scope:**

- [ ] `web/src/tasks/components/task-detail/execution/TaskDetailExecutionSection.tsx` — collapsible cycle list with phase rollups, status pills, attempt counter.
- [ ] Sub-components grouped under `task-detail/execution/` per [`docs/WEB.md`](./WEB.md) family-folder convention.
- [ ] Mounted on `TaskDetailPage` between prompt and updates timeline (least disruptive slot).
- [ ] Component + interaction tests under same family folder (`*.test.tsx`).
- [ ] Update `docs/WEB.md` module map.
- [ ] Update `web/src/app/styles/` with a new partial only if necessary; otherwise reuse pill/section tokens.

**Exit criteria:**

- `npm test -- --run`, `npm run lint`, `npm run build` all green.
- A user-visible screenshot or short note in commit body if UI is non-trivial.

**Commit:** `web: render task execution cycles and phases on task detail page`

**STOP — ask permission to begin Stage 9.**

---

### Stage 9 — Final integration sweep

**Scope:**

- [ ] Full `./scripts/check.ps1` (no skip flags).
- [ ] `funclogmeasure -enforce` across the whole repo.
- [ ] Re-read `docs/EXECUTION-CYCLES.md`, `docs/API-HTTP.md`, `docs/API-SSE.md` for drift introduced by later stages.
- [ ] Append a final session row to `.agent/backend-improvement-agent.log` summarising the slice and tagging followups in `### Notes / followups` below.

**Exit criteria:**

- All checks green.
- This file updated: every checkbox checked, followups extracted, plan archived (or marked complete in `docs/AGENTIC-LAYER-PLAN.md`).

**Commit:** `chore: finalize execution cycles slice (full check pass + docs sweep)`

---

## Common verification

| Before commit (per stage) | Command |
|---|---|
| Go-only stages (1–6) | `go vet ./... ; go test ./... -count=1 ; go run ./cmd/funclogmeasure -enforce` |
| Web stages (7–8) | `cd web ; npm test -- --run ; npm run lint ; npm run build` |
| Docs-only stage (6) | `$env:CHECK_SKIP_WEB='1' ; .\scripts\check.ps1` |
| Full pass (Stage 9) | `.\scripts\check.ps1` |

`gofmt -w` on touched `*.go` files always.

## What's deliberately deferred (not scope)

- `task_cycle_artifacts` table — keep artifacts in `details_json` until UI demands a browser.
- Cross-cycle dependencies — wait until multiple workers exist.
- Worker process itself (V1 in `docs/AGENTIC-LAYER-PLAN.md`) — this plan only builds the **substrate** the worker will use. The worker is its own slice.
- Retry/backoff policy — worker-side concern, not data-model concern.
- Visual cycle graph / Gantt — UI panel in Stage 8 is a list, not a chart.

## Notes / followups

(Populated as stages discover incidental work — keep this section as the catch-all so individual stages stay scoped.)

- _none yet_

## Status

| Stage | State | Commit |
|---|---|---|
| 0 — Plan | in progress | — |
| 1 — Domain | pending | — |
| 2 — Schema + CRUD | pending | — |
| 3 — Dual-write mirror | pending | — |
| 4 — Handler | pending | — |
| 5 — SSE | pending | — |
| 6 — Docs | pending | — |
| 7 — Web data layer | pending | — |
| 8 — Web UI panel (optional) | pending | — |
| 9 — Integration sweep | pending | — |
