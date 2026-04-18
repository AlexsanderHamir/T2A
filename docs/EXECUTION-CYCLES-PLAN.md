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

- [x] Add `Phase` enum (`diagnose`, `execute`, `verify`, `persist`) in `enums.go`.
- [x] Add `CycleStatus` enum (`running`, `succeeded`, `failed`, `aborted`).
- [x] Add `PhaseStatus` enum (`running`, `succeeded`, `failed`, `skipped`).
- [x] Add 7 new `EventType` constants: `cycle_started`, `cycle_completed`, `cycle_failed`, `phase_started`, `phase_completed`, `phase_failed`, `phase_skipped`. (Constants only; nothing emits them yet.)
- [x] Add `TaskCycle` and `TaskCyclePhase` GORM model structs in `models.go`. Tags written but no `AutoMigrate` registration yet.
- [x] Add new file `cycle_state.go` with `func ValidPhaseTransition(prev, next Phase) bool` and `func TerminalCycleStatus(s CycleStatus) bool` / `func TerminalPhaseStatus(s PhaseStatus) bool`.
- [x] Add `cycle_state_test.go` covering: valid forward transitions, valid `verify → execute` re-entry, all invalid transitions rejected, terminal-status helper truth tables, enum string-value drift guards, mirror events excluded from `EventTypeAcceptsUserResponse`.
- [x] Add `Scan` / `Value` methods for the three new enums in `sqltypes.go` so GORM can persist them (Stage 2 requires this).
- [x] Update `pkgs/tasks/domain/doc.go` to mention the new types and helpers.

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

- [x] Register `domain.TaskCycle` and `domain.TaskCyclePhase` in `pkgs/tasks/postgres/postgres.go::Migrate` `AutoMigrate` call.
- [x] Decide partial-unique-index strategy for "one running cycle per task" + "one running phase per cycle":
  - **Chosen:** in-TX `SELECT ... LIMIT 1` guard inside store (portable across Postgres + SQLite). Recorded under [Notes / followups](#notes--followups) for a future Postgres-only partial-unique index migration.
- [x] New file `pkgs/tasks/store/store_cycles.go` with: `StartCycle`, `TerminateCycle`, `GetCycle`, `ListCyclesForTask`.
- [x] New file `pkgs/tasks/store/store_cycle_phases.go` with: `StartPhase`, `CompletePhase`, `ListPhasesForCycle`.
- [x] Validate inputs at the store boundary (status enum, phase enum, transition validity via `domain.ValidPhaseTransition`); map invalid input to `domain.ErrInvalidInput`.
- [x] New file `pkgs/tasks/store/store_cycles_test.go` (table-driven) — happy path + all invariant violations.

**Out of scope for this stage:** `task_events` mirror writes. Pure cycle/phase state writes only — keeps the diff small and the patterns pure.

**Exit criteria:**

- [x] `go test ./pkgs/tasks/store/... -count=1` passes.
- [x] `go test ./pkgs/tasks/postgres/... -count=1` passes (AutoMigrate still works on SQLite).
- [x] `internal/tasktestdb` test fixture still opens cleanly with the new tables.
- [x] `funclogmeasure -enforce` clean (392/392 functions covered).

**Commit:** `store: add task_cycles + task_cycle_phases tables and CRUD operations` (SHA recorded once pushed).

**STOP — ask permission to begin Stage 3.**

---

### Stage 3 — Dual-write mirror to `task_events`

**Scope:**

- [x] Inside each public store function from Stage 2, append the corresponding mirror `task_events` row in the **same `gorm.DB` transaction** (`StartCycle` → `cycle_started`; `TerminateCycle` → `cycle_completed` for `succeeded`, `cycle_failed` for `failed`/`aborted` with the original status preserved in payload; `StartPhase` → `phase_started`; `CompletePhase` → `phase_completed`/`phase_failed`/`phase_skipped`).
- [x] Capture the assigned `task_events.seq` and write it back into the **phase** row's `event_seq` column (cycle row has no such column — recorded as a deliberate scope decision in [Notes / followups](#notes--followups)).
- [x] Add `pkgs/tasks/store/store_cycles_dualwrite_test.go` pinning the invariant table-driven across every entry point: payload shape, actor mirroring, `event_seq` backfill, monotonic `task_events.seq` across mixed operations, and a forced-failure case that proves the cycle insert is rolled back when the mirror append fails.
- [x] Confirm `EventTypeAcceptsUserResponse` (in `pkgs/tasks/domain/event_user_response.go`) does **not** include the seven new mirror types — they are observational, not interactive (assertion baked into the dual-write test file so future drift fails CI).
- [x] Surface change: `TerminateCycle`, `StartPhase`, and `CompletePhaseInput` now require an `Actor` (`by`) so the mirror row records who drove the transition. Stage 2 tests updated to match.

**Exit criteria:**

- [x] `go test ./pkgs/tasks/store/... -count=1` passes including the new dual-write invariant suite.
- [x] `go test ./pkgs/tasks/handler/... -count=1` still passes (no handler API change yet).
- [x] `go test ./... -count=1` green across the whole repo.
- [x] `funclogmeasure -enforce` clean (398/398 functions covered).

**Commit:** `store: mirror cycle and phase transitions into task_events in the same transaction` (SHA recorded once pushed).

**STOP — ask permission to begin Stage 4.**

---

### Stage 4 — HTTP handler routes

**Scope (touch only `pkgs/tasks/handler/` + `internal/taskapi/` for mux registration):**

- [x] New file `pkgs/tasks/handler/handler_cycles.go` exposing six routes:
  - `POST /tasks/{id}/cycles` — `Idempotency-Key` honored via global middleware; body `{parent_cycle_id?, meta?}` (actor sourced from `X-Actor`, not the body).
  - `GET  /tasks/{id}/cycles` — limit-based pagination with `has_more` envelope (`?limit=` 1–200, default 50). **Followup:** keyset pagination matching `/events` conventions once store gains cursor support.
  - `GET  /tasks/{id}/cycles/{cycleId}` — embedded `phases[]`.
  - `POST /tasks/{id}/cycles/{cycleId}/phases` — body `{phase}`.
  - `PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}` — body `{status, summary?, details?}`; state machine validates.
  - `PATCH /tasks/{id}/cycles/{cycleId}` — body `{status, reason?}`.
- [x] JSON DTOs colocated in `handler_cycles_json.go`; reject unknown fields and trailing data (existing handler convention).
- [x] `X-Actor` plumbed through to store via existing `actorFromRequest` helper for every cycle/phase mutation.
- [x] All abuse guards: 128-byte path segment caps for task/cycle IDs, 32-byte cap for `phaseSeq` and `limit` query, body field length caps consistent with current handler bar.
- [x] Cross-task ID mismatch protection: `assertCycleBelongsToTask` returns 404 when the cycle exists but belongs to a different task, preventing information leakage.
- [x] Mux registration in `handler.go` alongside other resource families.
- [x] Tests:
  - `handler_http_cycles_test.go` — happy paths for all six routes plus a dual-write invariant check that walks the `task_events` audit log via HTTP.
  - `handler_http_cycles_contract_test.go` — pins every documented 400 string, every JSON shape, response key set, content-type, status code, length caps, and a Stage-5 guardrail asserting **no** SSE events are emitted yet.

**Exit criteria:**

- [x] `go vet ./...` clean.
- [x] `go test ./... -count=1` passes.
- [x] `funclogmeasure -enforce` clean (412/412 functions covered).
- [x] New routes appear in `pkgs/tasks/handler/README.md` file-map.

**Commit:** `handler: add task execution cycle and phase REST routes with contract tests` (SHA recorded once pushed).

**STOP — ask permission to begin Stage 5.**

---

### Stage 5 — SSE trigger surface

**Scope:**

- [x] Add `task_cycle_changed` SSE event type with payload `{type: "task_cycle_changed", id: <task_id>, cycle_id: <cycle_id>}`. Implemented as a new `TaskCycleChanged` constant plus an opt-in `cycle_id` field on `TaskChangeEvent` (`omitempty` so existing payloads stay byte-identical).
- [x] Call `notifyCycleChange(taskID, cycleID)` from each cycle-mutating handler write path (`postTaskCycle`, `patchTaskCycle`, `postTaskCyclePhase`, `patchTaskCyclePhase`).
- [x] Extend `pkgs/tasks/handler/sse_trigger_surface_test.go` with subtests covering every new mutating route → assert exact published `{type, id, cycle_id}` set; extend the read-only routes subtest to include `GET /tasks/{id}/cycles` and `GET /tasks/{id}/cycles/{cycleId}`.
- [x] Update `docs/API-SSE.md` trigger table and add the `task_cycle_changed` payload example.
- [x] Retire the Stage-4 `TestHTTP_cycle_routes_emit_no_sse` guardrail (its job is now covered by the trigger-surface subtests).

**Exit criteria:**

- [x] All SSE trigger surface tests pass with new subtests included.
- [x] `docs/API-SSE.md` and the test stay in sync (docs PR contract).
- [x] `go vet ./...` clean, `go test ./... -count=1` green, `funclogmeasure -enforce` clean.

**Commit:** `handler: publish task_cycle_changed on cycle and phase mutations` (SHA recorded once pushed).

**STOP — ask permission to begin Stage 6.**

---

### Stage 6 — Backend docs + contract pinning ✅

**Scope (docs-only, no code changes):**

- [x] New `docs/EXECUTION-CYCLES.md` — design rationale, schema diagram, dual-write invariant, state machine diagram, "where reads go" table, concurrency rules, what's intentionally out.
- [x] `docs/API-HTTP.md` — added cycle routes to handler-routes table (new "Task execution cycles (`/tasks/{id}/cycles`)" section), pinned `taskCycleResponse` / `taskCyclePhaseResponse` JSON envelopes, added the cycle-routes 400 string subsection (covers POST/GET cycles, PATCH terminate, POST phase, PATCH phase including `phase_seq` path validation), cross-referenced `task_cycle_changed` SSE.
- [x] `docs/DESIGN.md` — added contract-doc row for `EXECUTION-CYCLES.md` and a new Limitation **15** explaining the cycles-vs-flat-audit dual-write rationale and the deliberate non-consolidation.
- [x] `docs/AGENT-QUEUE.md` — added "Workers and execution cycles" section and cross-links to `EXECUTION-CYCLES.md` / `AGENTIC-LAYER-PLAN.md`.
- [x] `docs/AGENTIC-LAYER-PLAN.md` — promoted the substrate note to point at `EXECUTION-CYCLES.md`, marked the substrate TODO under V1 as done, and refined the V1 scope to call the cycle routes by name.
- [x] `AGENTS.md` repo-map — added a dedicated "Execution cycles HTTP" row for `handler_cycles.go` + `handler_cycles_json.go`; extended the Persistence and Domain rows for the new store entrypoints and types; added `EXECUTION-CYCLES.md` to the read-order table.
- [x] `docs/README.md` index — new row for `EXECUTION-CYCLES.md` plus a "Where to put updates" entry covering the cycles substrate.

**Exit criteria:**

- `./scripts/check.ps1` with `CHECK_SKIP_WEB=1` (docs-only fast path).
- All cross-links resolve (manual scan).

**Commit:** `docs: document task execution cycles primitive and update API + design references` (SHA `bfa91c1`).

**STOP — ask permission to begin Stage 7.**

---

### Stage 7 — Web data layer

**Scope (touch only `web/src/api/`, `web/src/types/`, `web/src/tasks/task-query/`, `web/src/tasks/hooks/`):**

- [x] `web/src/types/cycle.ts` — TS types for `TaskCycle`, `TaskCyclePhase`, status/phase enums (plus list/detail envelopes and request bodies).
- [x] Re-export from `web/src/types/index.ts` barrel (`@/types`); extend `TaskChangeType` with `task_cycle_changed` and `TaskChangeEvent` with optional `cycle_id`.
- [x] `web/src/api/cycles.ts` — `listTaskCycles`, `getTaskCycle`, `startTaskCycle`, `terminateTaskCycle`, `startTaskCyclePhase`, `patchTaskCyclePhase` (mirrors `tasks.ts` conventions; reuses `assertTaskPathId` / `assertPositiveSeq`; threads `X-Actor` like the checklist routes).
- [x] Extend `web/src/api/parseTaskApi.ts` with `parseTaskCycle`, `parseTaskCyclePhase`, `parseTaskCyclesListResponse`, `parseTaskCycleDetail`; coverage in `parseTaskApi.test.ts` (status / phase / actor / date validation, optional fields, indexed errors).
- [x] Re-export from `web/src/api/index.ts`.
- [x] `web/src/tasks/task-query/queryKeys.ts` — `taskQueryKeys.cycles(taskId)` and `taskQueryKeys.cycle(taskId, cycleId)` scoped under `["tasks","detail",id,"cycles",...]` so a `task_updated` invalidation still sweeps cycles, but a `task_cycle_changed` does not invalidate the broader detail tree.
- [x] `web/src/tasks/task-query/sseInvalidate.ts` — added `parseTaskChangeFrame` returning a `task` / `cycle` discriminated union; `collectTaskIdFromSSEData` now skips cycle frames so they cannot accidentally invalidate the whole task subtree. `useTaskEventStream` accumulates two pending sets (broad task ids vs per-task cycle ids) and dedupes (cycle invalidation is suppressed when the same task already has a broad pending entry). Tests pin the granularity (`useTaskEventStream.test.tsx` asserts cycle frames invalidate only `["tasks","detail",task,"cycles"]`).
- [x] `web/src/tasks/hooks/useTaskCycles.ts` + colocated test (`useTaskCycles` for the list, `useTaskCycle` for the detail; tests cover happy path, `limit` query forwarding, disabled state, and 404 surfacing).

**Exit criteria:**

- `cd web && npm test -- --run` passes (435 tests).
- `npm run lint` clean.
- `npm run build` clean.

**Commit:** `web: add task cycles API client, query keys, SSE invalidation, and useTaskCycles hook` — `d5948d2`

**STOP — Stage 7 done. Awaiting decision on Stage 8 (optional UI panel) vs jumping to Stage 9 (final integration sweep).**

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

- [x] Full `./scripts/check.ps1` (no skip flags) — gofmt clean, `go vet` clean, all Go tests pass, web 70 files / 435 tests pass, eslint clean, vite build clean.
- [x] `funclogmeasure -enforce` across the whole repo — 145 files / 413 funcs / **100.0% slog coverage**.
- [x] Re-read `docs/EXECUTION-CYCLES.md`, `docs/API-HTTP.md`, `docs/API-SSE.md` for drift introduced by later stages. Two items found in `API-SSE.md` (Stage 7 drift): the "underlying primitive" cross-reference pointed at the plan instead of the contract doc, and the SPA-invalidation paragraph at the bottom predated the granular `task_cycle_changed` cycle path. Both fixed in this commit.
- [x] Appended Session 13 to `.agent/backend-improvement-agent.log` summarising the whole slice (Stages 0–9, all commit SHAs, verification numbers) and added the four execution-cycles followups (partial unique index, `TaskCycle.EventSeq`, keyset cursor for `/cycles`, optional UI panel) plus carried-forward queue items.

**Exit criteria:**

- [x] All checks green (full `./scripts/check.ps1` end-to-end, no skip flags).
- [x] This file updated: every checkbox checked, followups extracted to the agent log queue, slice marked complete in `docs/AGENTIC-LAYER-PLAN.md` (substrate stages 1–9 done; V1 worker remains its own slice).

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

- **(Stage 2)** Concurrency invariants ("at most one running cycle per task", "at most one running phase per cycle") are enforced today by an in-TX `SELECT ... LIMIT 1` guard in `pkgs/tasks/store/store_cycles.go` / `store_cycle_phases.go`. The portable approach was chosen because GORM `AutoMigrate` does not drive Postgres-only partial unique indexes. **Followup:** add a Postgres-only post-migration hook for `CREATE UNIQUE INDEX ... WHERE status = 'running'` once the schema lives in a real migration tool, then keep the in-TX guard for SQLite tests as a belt-and-braces backup.
- **(Stage 3)** Only `TaskCyclePhase` carries an `event_seq` backlink to `task_events`; `TaskCycle` does not. Rationale: the cycle's `started`/`terminated` mirror rows are easily reconstructed from `(task_id, type IN ('cycle_started','cycle_completed','cycle_failed') AND data_json->>'cycle_id' = ?)`, while phases have many transitions per row and benefit from a one-shot pointer to the *most recent* mirror. **Followup:** if a future read path proves expensive without the cycle backlink, add `TaskCycle.EventSeq` in a dedicated migration; for now, the indirection is cheap.
- **(Stage 3)** `TerminateCycle`, `StartPhase`, and `CompletePhaseInput` now require a `by domain.Actor` argument so the mirror row records who drove the transition. This is a pre-handler API change; Stage 4 will plumb `X-Actor` through to satisfy it.
- **(Stage 4)** `GET /tasks/{id}/cycles` ships with limit-based pagination (`?limit=` + `has_more`) instead of the keyset cursor pattern used by `/events`. The store layer does not yet expose a cursor for `task_cycles`. **Followup:** add `ListCyclesForTaskAfter(taskID, afterAttemptSeq)` and switch the handler to keyset paging once a UI consumer needs it.
- **(Stage 4)** Cycle/phase mutations do not publish SSE events yet — explicitly deferred to Stage 5 (`task_cycle_changed`). A guardrail test (`TestHTTP_cycle_routes_emit_no_sse`) pins this so an accidental Stage 5 leak fails CI.

## Status

| Stage | State | Commit |
|---|---|---|
| 0 — Plan | done | `c495148` |
| 1 — Domain | done | `31c9153` |
| 2 — Schema + CRUD | done | `f72ad84` |
| 3 — Dual-write mirror | done | `bd195fa` |
| 4 — Handler | done | `9151a58` |
| 5 — SSE | done | `0b2be37` |
| 6 — Docs | done | `bfa91c1` |
| 7 — Web data layer | done | `d5948d2` |
| 8 — Web UI panel (optional) | skipped | — (deferred; mirror `task_events` already render via `TaskUpdatesTimeline`. Followup tracked in agent log queue.) |
| 9 — Integration sweep | done | `b109316` |

**Slice complete.** All four execution-cycles followups are tracked in `.agent/backend-improvement-agent.log` Session 13 NEXT_SESSION_QUEUE: (1) Postgres-only partial unique index for `running` cycles/phases; (2) optional `TaskCycle.EventSeq` backlink if a future read path proves expensive; (3) keyset pagination for `GET /tasks/{id}/cycles` once a UI consumer needs >1 page; (4) optional `TaskDetailExecutionSection` UI panel once a worker actually drives cycles in production. Substrate is ready for the V1 worker (see [AGENTIC-LAYER-PLAN.md](./AGENTIC-LAYER-PLAN.md)).
