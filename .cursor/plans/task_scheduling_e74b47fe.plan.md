---
name: Task Scheduling — Operator-Controlled Agent Pickup Time
overview: "Give operators end-to-end control over when the agent worker is allowed to pick up a task. The substrate (tasks.pickup_not_before column, agent WHERE clause, RFC3339 wire field) already exists from the earlier global agent_pickup_delay_seconds work; the gap is (a) letting users set/edit/clear the time per task from create / detail / bulk-list, (b) closing a real race in the in-memory notifier that lets the worker pick up a future-scheduled task immediately, and (c) showing scheduled times correctly in an operator-chosen timezone with a low-friction quick-pick UI."
todos:
  - id: stage0_race_fix
    content: "Stage 0 (prereq): close the notifyReadyTask race so a task with pickup_not_before in the future is NOT pushed to the in-memory queue until the time has passed; add a regression test that fails on main today."
    status: pending
  - id: stage1_settings_tz
    content: "Stage 1: add display_timezone IANA string to app_settings (domain + store + handler + SettingsPage), default 'UTC', validated against time.LoadLocation."
    status: pending
  - id: stage2_api_create_edit
    content: "Stage 2: accept pickup_not_before on POST /tasks and PATCH /tasks/{id} (currently server-set only); validate parse + UTC + reject pre-2000 sentinel; document in API-HTTP.md."
    status: pending
  - id: stage3_create_modal_ui
    content: "Stage 3: create-modal 'Schedule for' field with native datetime-local + quick-pick chips ('In 1 h', 'Tonight 9pm', 'Tomorrow 9am', 'Next Monday 9am', 'Clear'); render in app timezone; clear == null."
    status: pending
  - id: stage4_detail_edit_ui
    content: "Stage 4: TaskDetailPage 'Scheduled' badge + edit/clear control reusing the same picker; PATCH integrates with optimistic flow; SSE task_updated already covers refresh."
    status: pending
  - id: stage5_bulk_reschedule
    content: "Stage 5: task list multi-select + 'Reschedule' bulk action calling PATCH N times with shared concurrency cap; status filter learns a 'Scheduled' bucket (status=ready AND pickup_not_before > now)."
    status: pending
  - id: stage6_observability
    content: "Stage 6: surface 'Scheduled (deferred)' as a KPI on the observability overview + new agent idle reason 'awaiting_scheduled_task' when ready queue is empty only because every ready task is in the future."
    status: pending
isProject: false
---

## Goals & non-goals

**Goals**
- An operator can pick a future time when filing a task, change it later, clear it, or reschedule a batch from the list — all without dropping into SQL.
- The agent worker honours that time end-to-end: not just in the reconcile sweep (already true) but also in the immediate notify path (currently broken).
- Times render in an operator-chosen timezone (decision: explicit `display_timezone` in `app_settings`, default `UTC`) and travel as RFC3339 UTC on the wire — same convention as every other timestamp.
- Native, dependency-free UI with quick-pick chips so 90 % of schedules require zero typing.

**Non-goals (this plan)**
- **Recurrence** (cron / "every Monday"): out of scope per decision `recurrence=oneshot`. Schema is single-shot only; if recurrence comes later it will be a new `task_schedules` side-table, not a column reshape, so this plan is forward-compatible.
- **Visibility hiding**: per decision `scope=agent_only`, scheduled tasks remain fully visible in the default task list. We add a *Scheduled* status filter as a UX affordance, but the row is never hidden.
- **Per-task missed-window expiry**: per decision `expiry=queue_normally`, a missed schedule just queues normally when the worker is next free. No `missed` state, no `expires_at`.
- **Calendar view / Gantt**: out of scope. The list is the surface.

## Decisions locked in (from the design Q&A)

| Decision | Choice | Implication |
|---|---|---|
| Scheduling scope | **Agent-pickup only** | Single field `pickup_not_before`; row stays visible. |
| When operator can set | **Create + edit + bulk** | API accepts on POST/PATCH; UI in 3 places. |
| Recurrence | **One-shot only** | No cron column; schema stays single-shot. |
| Missed-window behaviour | **Queue normally** | No new status; no `expires_at` column. |
| UI input | **Native datetime-local + quick-pick chips** | Zero new deps. Roughly +200 LoC of TSX/CSS. |
| Notifier race | **Fix in plan (Stage 0)** | Real correctness bug, not cosmetic. |
| Timezone | **Explicit `display_timezone` in app_settings** | New IANA-string field; UI renders in that zone; wire stays UTC. |

---

## Substrate that already exists (from scout)

These are NOT being added — they're being leveraged:

- `domain.Task.PickupNotBefore *time.Time` (`pickup_not_before` column, indexed) — `pkgs/tasks/domain/models.go`.
- `pickup_not_before` is parsed and exposed on the frontend `Task` type — `web/src/types/task.ts`.
- The agent's `ready.ListQueueCandidates` already filters `pickup_not_before IS NULL OR pickup_not_before <= now()` — `pkgs/tasks/store/internal/ready/ready.go:72`.
- `app_settings.agent_pickup_delay_seconds` already defers brand-new tasks globally — `pkgs/tasks/handler/handler_task_crud.go:62`.
- SSE `task_updated` is the catch-all change frame and already invalidates the right query keys — no new event type needed.

What's missing today:

1. The wire contract for **POST/PATCH** doesn't accept `pickup_not_before` (only the server can set it via the global delay).
2. There is **no UI** to set/edit/clear it.
3. **`facade_tasks.go.notifyReadyTask`** pushes the task onto the in-memory queue immediately on create regardless of `PickupNotBefore` — so a brand-new ready task with a future pickup time can be picked up by the worker before the SQL filter ever sees it. (Reconcile would re-queue it correctly, but the create-time push wins the race.)
4. There's no concept of an operator timezone.

---

## Stage 0 — Notifier race fix (PREREQ, ships first)

**Why first**: every later stage trusts that "I set pickup_not_before=T+1h on create" actually delays the agent. Today it doesn't if the worker is idle when the task is created. Build scheduling on a known-good substrate.

**Backend changes**

- `pkgs/tasks/store/facade_tasks.go`:
  - In `Create` and `Update`, gate the `notifyReadyTask` call on `t.PickupNotBefore == nil || !t.PickupNotBefore.After(time.Now().UTC())`. If the time is in the future, don't push to the in-memory queue — let the **reconcile loop** pick it up after the time has passed (it already runs every `UserTaskAgentReconcileInterval`, default 5 min).
  - Add a doc comment explaining the invariant: *the in-memory queue must never contain a task that the SQL filter would reject.*

**Tests**

- New unit test in `pkgs/tasks/store/facade_tasks_test.go` (or the existing test file beside it):
  - `TestStore_Create_doesNotNotifyWhenPickupInFuture`: create a task with `PickupNotBefore = now+1h`, assert the notifier was NOT called.
  - `TestStore_Create_notifiesWhenPickupAlreadyPassed`: same, with `PickupNotBefore = now-1m`, assert notifier IS called.
  - `TestStore_Update_doesNotNotifyOnReadyTransitionWhenPickupInFuture`: similar, on the Update path.

**Reconcile interaction**

- The reconcile loop already calls `ready.ListQueueCandidates` which has the correct WHERE clause. No change needed there. We just need to make sure the loop is *enabled* in production (env `USER_TASK_AGENT_RECONCILE_INTERVAL`); document the relationship between the two paths in the doc comment.

**Documentation**

- One paragraph in `docs/SCHEDULING.md` (new file, used by every later stage) on "the two queues" — the SQL queue (authoritative, time-correct) and the in-memory queue (fast path, must mirror SQL filter).

**Acceptance**

- Regression test in step 2 fails on `main` today and passes on this branch.
- Existing supervisor + worker tests still pass (no behavioral change for tasks without `pickup_not_before`).

---

## Stage 1 — `display_timezone` in app_settings

**Why this stage exists**: the user picked `explicit_tz`. Every later stage's UI renders schedules in this zone. Doing it now means the create-modal in Stage 3 already has the formatter ready.

**Backend changes**

- `pkgs/tasks/domain/app_settings.go`:
  - Add `DisplayTimezone string `gorm:"not null;default:'UTC'"` with a doc comment: *IANA timezone identifier (e.g. America/New_York). Validated server-side via `time.LoadLocation` on PATCH; stored as the canonical name returned by the lookup. UI uses this for all human-facing time rendering. Wire format for every timestamp stays RFC3339 UTC.*
  - Add `DisplayTimezone: "UTC"` to `DefaultAppSettings()`.

- `pkgs/tasks/store/internal/settings/settings.go`:
  - Add `DisplayTimezone *string` to `Patch`; include in `IsEmpty()`; apply in `applyPatch()`.

- `pkgs/tasks/handler/handler_settings.go`:
  - Add `DisplayTimezone string `json:"display_timezone"` to `settingsResponse`; `*string `json:"display_timezone,omitempty"` to `settingsPatchBody`.
  - In the PATCH validator, when `DisplayTimezone` is non-nil and non-empty: call `time.LoadLocation(*body.DisplayTimezone)`; on error return 400 with `{"error":"invalid_timezone","detail":"..."}`. Empty string is rejected — to "reset" you PATCH to `"UTC"`.

**Frontend changes**

- `web/src/api/settings.ts`: add `display_timezone: string` to `AppSettings` and `*string` to `AppSettingsPatch`. `assertSettings` defaults to `"UTC"` if missing (backwards compatible).
- `web/src/settings/SettingsPage.tsx`: new `<select>` populated from `Intl.supportedValuesOf("timeZone")` (Chrome 99+/Firefox 93+/Safari 15.4+ — verify via `typeof Intl.supportedValuesOf === "function"`, fall back to a curated short list of ~30 common zones if unsupported).
- New helper `web/src/shared/time/appTimezone.ts`:
  - `useAppTimezone()` — reads from `useAppSettings`, returns `string` (default `"UTC"`).
  - `formatInAppTimezone(iso: string, tz: string, opts?: Intl.DateTimeFormatOptions): string` — wraps `new Intl.DateTimeFormat(undefined, { timeZone: tz, ... }).format(...)`.
- Comprehensive vitest for `appTimezone.ts` covering UTC, `America/New_York`, DST boundaries, invalid TZ fallback, and the `Intl.supportedValuesOf` capability check.

**Tests**

- Backend contract test: GET returns `"display_timezone":"UTC"` by default; PATCH with `"America/New_York"` round-trips and emits `settings_changed` SSE; PATCH with `"NotARealZone"` returns 400.
- Frontend SettingsPage test: change the timezone, save, and re-render — every timestamp on the page (the `updated_at` line is the only one today, but later stages add more) reflects the new zone.

**Acceptance**

- Settings page has a working timezone selector that survives reload.
- `display_timezone` appears in `GET /settings` and is documented in `docs/SETTINGS.md`.
- All existing settings tests still pass; no other timestamp display changes (we wire the helper now, use it in Stages 3+).

---

## Stage 2 — API: accept `pickup_not_before` on create + edit

**Why this stage exists**: Stages 3–5 are pure UI on top of this. Without it, the modal has nothing to send.

**Backend changes**

- `pkgs/tasks/handler/handler_task_json.go`:
  - Add `PickupNotBefore *string `json:"pickup_not_before,omitempty"` to `taskCreateJSON` and the analogous patch struct (find the patch type used by `PATCH /tasks/{id}`).
  - Validation rules:
    - `nil` (omitted) on create → keep the existing global-delay behaviour (no change).
    - `nil` (omitted) on PATCH → don't touch the column.
    - `""` (explicit empty string) → set the column to `NULL` (operator clearing the schedule). PATCH only; create rejects empty.
    - Non-empty: `time.Parse(time.RFC3339, *v)`. On error → 400 `invalid_pickup_not_before`.
    - Parsed time must be **after the year 2000** (sentinel rejection); otherwise 400.
    - Parsed time may be in the past — that's just a no-op deferral and should be allowed (operators recovering from a typo).

- `pkgs/tasks/handler/handler_task_crud.go`:
  - In `create`: if the body provides `pickup_not_before`, **bypass the global `agent_pickup_delay_seconds`** (operator's explicit choice wins). Document this precedence rule.
  - On PATCH: pass the parsed `*time.Time` (or explicit-nil for clear) into the existing UpdateTaskInput. The column already exists.

- `pkgs/tasks/store/internal/tasks/`:
  - Update `CreateTaskInput` and `UpdateTaskInput` if they don't already carry `PickupNotBefore` (scout suggests Update may not) — add the field, plumb through `Create`/`Update`. For Update, follow the existing `*string`-vs-explicit-nil pattern used by other nullable patches in the same file (look at how `parent_id` clear is implemented and mirror it).

**Tests**

- `pkgs/tasks/handler/handler_http_create_contract_test.go` extensions:
  - `TestHTTP_Create_acceptsPickupNotBefore_overrideGlobalDelay` — set delay=60s globally, POST with `pickup_not_before=now+1h`, assert the response carries the explicit time and not now+60s.
  - `TestHTTP_Create_rejectsMalformedPickupNotBefore` — sends `"yesterday"`, expects 400.
  - `TestHTTP_Create_rejectsPre2000PickupNotBefore` — sentinel guard.

- `pkgs/tasks/handler/handler_http_patch_contract_test.go` (or whatever the patch contract test file is):
  - `TestHTTP_Patch_setsPickupNotBefore`.
  - `TestHTTP_Patch_clearsPickupNotBeforeWithEmptyString`.
  - `TestHTTP_Patch_emitsTaskUpdatedSSE_onScheduleChange`.

- One end-to-end test exercising the Stage 0 invariant via HTTP: POST with `pickup_not_before=now+5s`, assert no `task_event` of type "agent picked up" before T+5s, then advance the test clock and assert pickup happens.

**Frontend changes (just the API client)**

- `web/src/api/tasks.ts`:
  - Add `pickup_not_before?: string | null` to the create + patch input types.
  - Encoding rule documented in JSDoc: pass `null` to clear, pass `string` (RFC3339 UTC) to set, omit to leave unchanged.

**Acceptance**

- All four new contract tests pass.
- `docs/API-HTTP.md` updated for both endpoints with examples.
- A `curl` PATCH from the terminal can set, change, and clear a schedule; SSE consumers receive a `task_updated` for each.

---

## Stage 3 — Create-modal "Schedule for" UI

**Components**

- New `web/src/shared/time/SchedulePicker.tsx`:
  - Composes a `<input type="datetime-local">` with five quick-pick `<button>`s: **In 1 hour**, **Tonight 9 PM**, **Tomorrow 9 AM**, **Next Monday 9 AM**, **Clear**.
  - Renders the current value in the **app timezone** from Stage 1 (`useAppTimezone()`).
  - Internally always works in UTC for the value emitted to the parent (`onChange(iso: string | null)`).
  - Quick-pick maths anchored on `Date.now()` (mockable in tests via `vi.spyOn(Date, "now")`).
  - Optional caption beneath: `"Agent will pick up at <formatted time> (<tz>)"` or `"Picks up immediately when the worker is free"` if value is null.
  - Accessible: `<fieldset>` + `<legend>`; quick-picks are `<button type="button">` so they don't submit the form; `aria-describedby` ties the caption to the input.

- `web/src/tasks/components/task-create-modal/TaskCreateModal.tsx`:
  - Add `<SchedulePicker value={schedule} onChange={setSchedule} />` near the bottom of the form, above the submit row.
  - Include `pickup_not_before: schedule` in the create payload (`null` when unset; the API client handles encoding).

- `web/src/tasks/hooks/useTasksApp.ts`:
  - New `schedule` state (RFC3339 string or null), `setSchedule`, reset on modal close.

**Visual / interaction notes**

- The datetime-local input shows naive local time (no zone). The caption is the source of truth for the operator (it shows "America/New_York" or whatever they chose in settings). To avoid the classic "I picked 9 AM but it stored 9 AM UTC" trap, the picker:
  1. Reads its `value` prop (UTC ISO),
  2. Converts it to the app TZ using `Intl.DateTimeFormat` parts for `value` attribute presentation (`YYYY-MM-DDTHH:mm` in the chosen zone),
  3. On user input, converts the naive local-time entry back to UTC using a `zonedTimeToUtc` helper. **No external dep**: implement via `Intl.DateTimeFormat` getOffset trick (well-trodden pattern; vitest covers DST forwards/backwards).

**Tests**

- `SchedulePicker.test.tsx`:
  - Renders empty when value is null; caption says "immediately when the worker is free".
  - Each quick-pick produces the correct ISO (with `Date.now` mocked + app TZ set to `America/New_York` to verify zone math).
  - Manual typing into the input emits the correct UTC ISO.
  - "Clear" emits `null` and the input goes blank.
  - DST forward and backward: a 9 AM local entry on the spring-forward day produces the correct UTC offset.

- `TaskCreateModal.test.tsx`: extend to assert that selecting a quick-pick and submitting POSTs `pickup_not_before` in the body.

**Acceptance**

- Operator can create a task with a future pickup time; the agent does not pick it up before that time (covered by Stage 0's invariant); the SettingsPage timezone change immediately re-renders the picker's caption and current value.

---

## Stage 4 — TaskDetailPage edit + clear + visible badge

**Components**

- New `web/src/tasks/components/task-detail/schedule/TaskDetailSchedule.tsx`:
  - Renders nothing when `task.pickup_not_before` is null AND the task is in a terminal state.
  - Otherwise renders a small panel: badge ("Scheduled for 2026-04-22 09:00 EDT"), an **Edit** button (opens `SchedulePicker` in a small inline disclosure or modal — match the existing edit-text-criterion modal pattern from `TaskDetailChecklistSection`), and a **Clear** button (calls PATCH with `pickup_not_before: null`).
  - Live-updates from SSE `task_updated` (already invalidates `taskQueryKeys.detail`, no new wiring).
  - Uses `useTaskPatchFlow` so it inherits the existing optimistic update path from the realtime-smoothness work.

- `TaskDetailPage.tsx`: mount the schedule panel inside the existing `TaskDetailHeader` block (visually adjacent to the status/priority pills).

**Tests**

- `TaskDetailSchedule.test.tsx`:
  - Hidden for a `done` task with no schedule.
  - Shown for a `ready` task with a schedule; badge text formatted in the app timezone.
  - Clicking **Clear** sends PATCH `pickup_not_before: null` exactly once and updates the badge.
  - Clicking **Edit** + selecting a quick-pick + Save sends PATCH with the new ISO.
  - Receives an SSE-driven update without a refetch storm (one invalidation, one refetch).

**Acceptance**

- Operator can change or clear the schedule from the detail page; the agent observes the new time within one reconcile interval (already true from Stages 0+2).

---

## Stage 5 — Bulk reschedule from the task list + "Scheduled" filter

**Why combined**: both depend on the list page learning about schedule semantics; doing them together amortises the per-row checkbox plumbing.

**Components**

- `web/src/tasks/components/task-list/section/TaskListSection.tsx`:
  - Add row-level checkboxes (sticky leftmost column). State: `selectedIds: Set<string>` in section-local state (we deliberately do NOT lift to URL — bulk selection is ephemeral by design).
  - Header checkbox toggles select-all-visible.
  - When `selectedIds.size > 0`, show a sticky bottom action bar: "N selected · Reschedule · Clear schedule · Cancel".
  - "Reschedule" opens a `SchedulePicker` modal (same component); on submit, fires N parallel PATCHes through `useTaskPatchFlow` with a concurrency cap of 5 (use `pLimit`-style local helper to avoid N=200 thundering herd).
  - "Clear schedule" PATCHes `pickup_not_before: null` for every selected row that has one set; a confirmation step ("Clear schedule on N tasks?") if N > 5.

- `web/src/tasks/components/task-list/filters/`:
  - `taskListFilterSelectOptions.ts`: add a synthetic option `"scheduled"` to the status filter alongside the real statuses, with label "Scheduled (deferred)".
  - `taskListClientFilter.ts`: when the filter is `"scheduled"`, match `task.status === "ready" && task.pickup_not_before && Date.parse(task.pickup_not_before) > Date.now()`.

**UX guardrails**

- Bulk PATCH doesn't show a separate confirm for each task; one combined error toast aggregates failures (e.g., "3 of 12 reschedules failed: …") so operators see the wood and the trees.
- Selection state clears on filter change, sort change, or successful bulk action — preventing the classic "I selected 12, applied filter, now 'Apply to selection' targets things I can't see".

**Tests**

- `TaskListSection.test.tsx`:
  - Selecting 3 rows + Reschedule + a quick-pick fires exactly 3 PATCHes with the same ISO body.
  - Selecting 3 rows + Clear schedule fires 3 PATCHes with `pickup_not_before: null`.
  - One failing PATCH out of three results in a single toast that lists the failed task ID and leaves the other two PATCHes' optimistic updates in place.
  - Filtering by "Scheduled" hides currently-running tasks and tasks whose pickup time has passed.

**Acceptance**

- An operator can reschedule (or unschedule) ten tasks in three clicks.
- The filter answers the question "show me everything queued for the future".

---

## Stage 6 — Observability surface

**Why this exists**: until the operator can *see* "12 tasks queued for the future, none ready right now", the difference between "system idle because nothing to do" and "system idle because operator scheduled everything for later" is invisible.

**Backend changes**

- `cmd/taskapi/run_agentworker.go`:
  - `decideIdle()` already returns reasons. Add a new check at the bottom (after all current checks pass and the runner probe succeeds): if `ready.ListQueueCandidates` returns empty AND there exists at least one row with `status='ready' AND pickup_not_before IS NOT NULL AND pickup_not_before > now()`, return idle reason `"awaiting_scheduled_task"`.
  - Add the new reason string to the documented set in the file header comment.

- `pkgs/tasks/handler/handler_task_stats.go` (or wherever `GET /tasks/stats` lives):
  - Add `scheduled_count int` to the response = count of (`status=ready AND pickup_not_before > now`).
  - One efficient query (single COUNT with WHERE), no extra round-trip.

**Frontend changes**

- `web/src/types/task.ts`: add `scheduled_count: number` to `TaskStatsResponse`; parser defaults to 0 for backwards compatibility.
- `web/src/observability/ObservabilityOverview.tsx`: new KPI card "Scheduled (deferred)" between "Ready" and "In flight". Caption: "queued for a future time".
- The new KPI inherits the Stage 1 timezone for any timestamp it might surface (none today, but the pattern is set for hover-tooltips later).

**Tests**

- Backend contract test: stats response includes `scheduled_count` reflecting the seeded fixture.
- Backend supervisor test: when the only ready task has `pickup_not_before` in the future, `decideIdle()` returns `awaiting_scheduled_task`.
- Frontend test: the KPI card renders the count and is `aria-busy` while the stats query is pending.

**Acceptance**

- The observability page distinguishes "0 ready, 0 scheduled" (truly idle) from "0 ready, 12 scheduled" (intentionally deferred).
- The new idle reason appears in the existing `effectiveSettingsLog` slog output so operators reading logs see it too.

---

## Cross-cutting concerns

### Backwards compatibility
- Every new field (`display_timezone`, `pickup_not_before` on the wire, `scheduled_count`) defaults to a safe value (`"UTC"`, `null`/omitted, `0`). Older frontends ignore unknown keys; older backends that don't send them parse correctly via the documented defaults.

### Migration risk
- `display_timezone` is a single new column with a NOT NULL default. GORM AutoMigrate handles this for SQLite/Postgres without a manual SQL step. No backfill needed (default applies on read for existing rows).
- `pickup_not_before` already exists; no schema change for the core feature.

### SSE
- Reuses the existing `task_updated` and `settings_changed` frames. No new event type, no new ring-buffer slot, no client-side wiring beyond what `useTaskEventStream` already does.

### Observability of the feature itself
- Slog: every PATCH that sets/clears `pickup_not_before` emits a structured log line with the old + new value, the actor, and the resulting `pickup_not_before_at_unix` so it's queryable in a log search.
- Prometheus: bump the existing `taskapi_tasks_created_total` and `taskapi_tasks_updated_total` counters with a new label `had_schedule="true|false"` so we can see adoption over time.

### Accessibility
- `SchedulePicker` is a labelled `<fieldset>`; quick-picks are real buttons; the caption uses `aria-live="polite"` so screen-reader users hear "Scheduled for …" updates without being interrupted.
- Bulk action bar respects `prefers-reduced-motion`; the sticky bar slides in only when motion is allowed.
- Timezone selector uses the native `<select>` for built-in keyboard semantics.

### Security
- `display_timezone` is validated against `time.LoadLocation` server-side — no path traversal or injection vector.
- `pickup_not_before` is parsed with `time.Parse(time.RFC3339, ...)` and rejected if pre-2000 (defends against `"0001-01-01T00:00:00Z"` sentinel sneaking in as "no schedule" and bypassing the explicit-nil clear path).

### Performance
- The new `scheduled_count` query in stats is `COUNT(*) WHERE status='ready' AND pickup_not_before > NOW()` — already indexed via the existing `pickup_not_before` index.
- Bulk reschedule's concurrency cap (5) bounds the worst-case load on PATCH for a 200-row selection.

### Out-of-band correctness
- Stage 0's race fix removes the only known way the worker can pick up a task before its time. Stage 6's idle reason makes the *absence* of work due to scheduling visible, so the operator never wonders "why isn't anything running?" when the answer is "you told it not to".

---

## Sequencing & merge gates

1. **Stage 0** ships standalone (it's a bug fix; merging it doesn't require any UI). Two-way-door.
2. **Stage 1** ships standalone (settings only; no other surface depends on it merging first, but Stage 3+ depends on it being live).
3. **Stage 2** ships standalone but is gated behind Stage 0 in the merge train (otherwise newly-scheduled tasks via API would still race).
4. **Stages 3 → 4 → 5** ship in order; each is independently reviewable and shippable. Stage 5 depends on Stage 4 for the `useTaskPatchFlow` integration pattern.
5. **Stage 6** ships last; it depends on the prior stages' field being widely used to actually be informative.

### Execution discipline (autonomous, no pauses)

The operator will NOT be available to grant permission between stages. The agent executes all seven stages end-to-end in one continuous run.

- **Each stage is its own commit** (or a tight sequence of commits if the stage naturally splits — e.g. Stage 1 might be `1a backend` + `1b frontend`). One commit per logical, reviewable unit; never a single mega-commit spanning multiple stages.
- **Verify → commit → push, then move on. No pauses.** Order per stage:
  1. Run the stage's verification gate (`go test ./...` for backend-touching stages; `npm run lint && npm run build && npm run test` for frontend-touching stages; both for stages that touch both).
  2. If green, write the commit message (one-line `type(scope): subject` + body explaining the why), `git add` the stage's files, `git commit`, `git push`.
  3. Confirm `git status` is clean and the local branch is at parity with `origin/main`.
  4. Immediately begin the next stage in the same turn.
- **Never wait for human input.** The user's design decisions (locked in the table above) are sufficient for every implementation choice in every stage. If a genuinely new ambiguity appears mid-stage, the agent picks the option most consistent with the locked decisions, documents the choice in the commit body, and keeps moving — better to ship a defensible default than to stall.
- **When in doubt, pick the safest, least-complicated option, then write it down.** Any implementation choice that wasn't pre-decided by the user's locked decisions table follows this rule:
  1. **Safest** = smallest blast radius. Prefer additive over destructive (new column over reshape, new endpoint over breaking change, new component over refactor of an existing one). Prefer reversible over irreversible (feature flag over hard cutover; "preserve old data" over "delete on read"). Prefer well-trodden over clever (native HTML element over custom widget; standard library over a new dep).
  2. **Least-complicated** = fewest moving parts. If two designs both satisfy the requirement, pick the one with fewer files touched, fewer concepts introduced, fewer abstractions, and fewer test scaffolds. A boring solution shipped today is worth more than an elegant solution shipped next week.
  3. **Document it.** Every such choice is recorded in TWO places, no exceptions:
     - **Commit body**: a short paragraph under the heading `Decision:` naming the choice, the rejected alternative(s), and the one-sentence reason ("safest because rollback = `git revert`; simplest because zero new abstractions").
     - **`docs/SCHEDULING.md`** (the same doc Stage 0 introduces): an append-only "Implementation decisions" section with one bullet per choice, dated, linking the commit SHA. This becomes the audit trail when someone asks six months later "why was this built this way?".
  4. **No silent choices.** If a decision didn't make it into either the commit body or `docs/SCHEDULING.md`, treat it as a bug to be fixed in a follow-up commit within the same stage. The point is that the user, returning later, can read `docs/SCHEDULING.md` end-to-end and reconstruct every non-obvious call without spelunking through diffs.
- **Never skip the verification gate.** A red test bar means stop, fix, re-verify in the same stage's commit — do NOT push a known-red commit and "fix it next stage". The push gate is the truth gate.
- **Never `--no-verify`, never force-push, never amend a pushed commit.** If a commit hits a hook failure, fix it in a follow-up commit within the same stage, not by rewriting history.
- **Update the plan's todos as you go.** Each stage's todo flips `pending → in_progress → completed` in the same turn it executes. The final assistant message reports every commit SHA pushed and `git rev-list --count origin/main..HEAD` to prove there's nothing left in the working tree.

## Estimated effort

| Stage | Backend LoC | Frontend LoC | Tests LoC | New files |
|---|---|---|---|---|
| 0 (race fix) | ~30 | 0 | ~120 | 0 |
| 1 (timezone) | ~60 | ~150 | ~200 | 2 |
| 2 (API) | ~120 | ~30 | ~250 | 0 |
| 3 (create UI) | 0 | ~280 | ~250 | 1 |
| 4 (detail UI) | 0 | ~140 | ~180 | 1 |
| 5 (bulk + filter) | 0 | ~250 | ~200 | 0 |
| 6 (observability) | ~80 | ~50 | ~150 | 0 |

Roughly **1 working day of focused implementation per stage** with the existing test discipline; total ~5–7 days end-to-end.

## Risks & mitigations

| Risk | Likelihood | Mitigation |
|---|---|---|
| Stage 0 fix surfaces a latent bug in the reconcile loop (e.g. it isn't actually running in some envs) | Low-medium | Stage 0's regression test asserts the worker eventually picks the task up after the time passes — if it doesn't, we discover the reconcile gap immediately rather than in production. |
| Browsers without `Intl.supportedValuesOf` (older Safari, embedded webviews) | Low | Static fallback list of ~30 common IANA zones; logged warning so we know if it's hit in prod. |
| DST forwards/backwards math errors in `SchedulePicker` | Medium | Dedicated test cases for both spring-forward and fall-back days in `America/New_York` and `Europe/London`. |
| Operators forget that schedule is in app TZ, not browser TZ | Medium | Every render of a scheduled time appends the TZ abbreviation (e.g. "EDT") in the caption; SettingsPage shows the current zone prominently. |
| Bulk reschedule with N=large fails partially and leaves an inconsistent UI state | Medium | Optimistic updates via `useTaskPatchFlow` already roll back on per-row error; aggregated error toast lists failed IDs so the operator can retry. |

