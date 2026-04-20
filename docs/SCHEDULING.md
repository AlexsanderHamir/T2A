# Task Scheduling — Operator-Controlled Agent Pickup Time

Operators can defer when the agent worker is allowed to pick up a task
by setting `tasks.pickup_not_before` (RFC3339 UTC). This document covers
the contract, the runtime invariants, and the implementation decisions.

> Cross-reference: `.cursor/plans/task_scheduling_e74b47fe.plan.md` is
> the executable plan; this doc is the durable, human-facing reference.

## Substrate

- `domain.Task.PickupNotBefore *time.Time` (column `pickup_not_before`,
  indexed) — see `pkgs/tasks/domain/models.go`. `nil` means "no
  deferral; pick up as soon as the worker is free".
- The wire encoding is RFC3339 UTC on `Task.pickup_not_before` (string
  or `null` to clear).
- App-wide default deferral: `app_settings.agent_pickup_delay_seconds`
  is added to `time.Now().UTC()` when a task is created in
  `status=ready` AND the operator did not pass an explicit
  `pickup_not_before` on the create body.

## The two queues

The agent worker is fed by two cooperating "queues" that MUST agree
on which tasks are eligible for pickup at any given moment:

1. **The SQL queue (authoritative).**
   `pkgs/tasks/store/internal/ready/ready.go:ListQueueCandidates`
   returns rows where `status='ready'` AND
   `(pickup_not_before IS NULL OR pickup_not_before <= now())`.
   This filter is evaluated on every reconcile sweep
   (default interval: `USER_TASK_AGENT_RECONCILE_INTERVAL`, 5 min) so
   a deferred task is *eventually* picked up at most one interval
   after its time arrives.

2. **The in-memory queue (fast path).**
   `Store.notifyReadyTask` pushes a task ID onto an in-process channel
   the worker drains continuously, so a task created via the API gets
   picked up in milliseconds rather than waiting for the next
   reconcile sweep. The notifier is fired by `Store.Create`,
   `Store.Update` (on a `ready` transition), and
   `Store.ApplyDevTaskRowMirror`.

**Invariant:** the in-memory queue MUST NEVER contain a task that the
SQL filter would currently reject. If it did, the worker would race
the reconcile sweep and pick up a deferred task immediately — the
exact regression Stage 0 of the scheduling plan closes.

`facade_tasks.go:shouldNotifyReadyNow(pickup, now)` is the single
gate enforcing this invariant; it returns `true` when `pickup` is
`nil` or `<= now`, mirroring the SQL filter byte-for-byte. The unit
test table `TestShouldNotifyReadyNow_unitTable` pins the boundary
(an "exactly now" pickup time notifies; "now+1s" does not).

If a deferred task is created or transitions into `ready` while its
pickup time is still in the future, the gate skips the in-memory push
and the task waits for the reconcile sweep — within at most
`USER_TASK_AGENT_RECONCILE_INTERVAL` of the time arriving. This is
the documented worst-case latency between "schedule arrives" and
"agent picks up".

## Operator workflow (forward reference)

The plan ships UI in stages 3–5: create-modal `SchedulePicker`,
detail-page edit/clear panel, and bulk reschedule from the list.
Until those stages land, schedules can only be set indirectly via
`agent_pickup_delay_seconds` (global) or via direct SQL.

## Implementation decisions

This section is **append-only** and dated. Each entry corresponds to a
non-obvious choice made during implementation — the kind a future
maintainer would otherwise have to reverse-engineer from diffs.
Format: `YYYY-MM-DD — [stage] — choice: rationale (commit SHA).`

- **2026-04-19 — [Stage 0] — `shouldNotifyReadyNow` lives in
  `facade_tasks.go` rather than `internal/notify`.**
  Rationale: the gate's correctness depends on `domain.Task` shape
  (specifically the `PickupNotBefore` field), which the public facade
  already owns. Pushing it down to `internal/notify` would force that
  package to import `domain` just to type-check a single field — a
  larger blast radius than keeping a 6-line private helper next to
  the only callers (`Create`, `Update`, `ApplyDevTaskRowMirror`).
  Rejected alternative: thread a `func(*time.Time) bool` predicate
  into `notify.Holder`. Simpler today; we can refactor when (and if)
  a third caller appears.

- **2026-04-19 — [Stage 0] — Strict `After` comparison ("exactly now"
  notifies).**
  Rationale: matches the SQL filter `pickup_not_before <= now()`
  byte-for-byte. Inverting either side would create a 1ns window
  where the two queues disagree.

- **2026-04-19 — [Stage 1] — Prepend "UTC" to
  `Intl.supportedValuesOf("timeZone")` in `supportedTimezones()`.**
  Rationale: `supportedValuesOf` returns the canonical IANA names and
  intentionally omits the legacy alias "UTC" (its canonical name is
  "Etc/UTC"). The backend's seed default is the literal string "UTC"
  (`domain.DefaultDisplayTimezone`); without prepending, a fresh
  install's SettingsPage would show no "UTC" option even though every
  timestamp on the page is currently in UTC. Operator-friendly UI
  trumps strict canonicalisation here. Rejected alternative: rewrite
  the backend default to "Etc/UTC" — that would gratuitously change
  the wire shape and the seed log line for every existing install.

- **2026-04-19 — [Stage 1] — `formatInAppTimezone` returns the input
  string verbatim on parse failure rather than empty.**
  Rationale: an unparseable timestamp is almost certainly a bug
  somewhere upstream (truncated string, wrong field). Showing the
  raw value gives the operator (and us) a fighting chance to spot
  the malformed payload during triage; silently rendering nothing
  hides the problem. Empty string is reserved for the "no value"
  case (null/undefined/empty), where blank IS the correct render.

- **2026-04-19 — [Stage 2] — Empty-string `pickup_not_before` on PATCH
  is treated as "clear" (symmetric with JSON `null`).**
  Rationale: the SchedulePicker UI in Stage 3 emits an empty string
  from a cleared `<input type="datetime-local">`. Treating it the
  same as JSON `null` means the SPA never has to special-case the two
  shapes when serializing the picker's emit value. The semantically
  cleaner alternative — reject empty string and require `null` — was
  rejected because every API client would then need a coalescing
  helper (`val === "" ? null : val`) at every call site. PATCH-only:
  on `POST /tasks` the empty string is **rejected** so a missing
  schedule on create has exactly one wire shape ("omit the field").

- **2026-04-19 — [Stage 2] — `pickup_not_before` changes do NOT emit
  a task-event audit row.**
  Rationale: scheduling is operator-facing **metadata**, not part of
  the task's narrative event log. The wire-level slog line on the
  HTTP handler (`debugHTTPRequest` with `patch_pickup_not_before`)
  IS the audit trail and is queryable in log search. Adding an
  `EventScheduleChanged` would force a new domain enum value, an
  SSE consumer wiring (today there is no consumer), and a doc
  update for `domain.EventType` — three new abstractions for a
  field that already round-trips through `task_updated`. Rejected
  alternative: emit the event anyway "for symmetry with
  `EventStatusChanged`". Symmetry isn't a goal in itself; the
  status change has external behaviour consequences (descendant
  done-checks, agent pickup eligibility) that justify a permanent
  audit row. A schedule change has none.

- **2026-04-19 — [Stage 2] — `Store.Update` notifies the in-memory
  ready queue when ANY pickup-touching PATCH lands on a `ready`
  task whose new `pickup_not_before` is "now or past" — not only on
  `prev != ready` transitions.**
  Rationale: clearing a future schedule (operator hits "Clear") on
  an already-ready task must wake the worker immediately; otherwise
  the task waits up to one reconcile interval (default 5 min) for
  no good reason. The narrower "transition only" gate from Stage 0
  was correct for `Create` and `Update`-into-`ready` but became
  insufficient once schedules are operator-mutable. The
  `shouldNotifyReadyNow` invariant is preserved: we only notify
  when the SQL filter would also accept the row.


- **2026-04-19 — [Stage 3] — The create-modal SchedulePicker treats
  the operator's chosen `display_timezone` as the only source of
  truth for the rendered wall-clock literal — never the host
  browser's zone.**
  Rationale: a native `<input type="datetime-local">` shows naive
  local time without a zone suffix. If we let the browser interpret
  that literal in the user's host zone, an operator in Tokyo
  configuring a fleet that displays in `America/New_York` would
  type `09:00` expecting NY morning and get Tokyo morning instead.
  Implementation: `isoToZonedDatetimeLocal`/`zonedDatetimeLocalToIso`
  in `web/src/shared/time/appTimezone.ts` round-trip through
  `Intl.DateTimeFormat` parts in the chosen zone. Caption
  underneath the input always shows the formatted instant in the
  app TZ so the operator sees what was actually scheduled.

- **2026-04-19 — [Stage 3] — Quick-pick chips never produce a
  no-op deferral into the past.**
  Rationale: an operator clicking "Tonight 9 PM" at 22:00 means
  "the next 21:00", not "an hour ago". `computeQuickPickIso` falls
  forward to tomorrow's 21:00 when today's 21:00 has already passed
  in the app TZ. Similarly, "Next Monday 9 AM" on a Monday goes
  to next week (+7 days), not today, because typing "Next Monday"
  on a Monday almost never means "today". DST forward + backward
  are covered by tests (spring-forward 2026-03-08 and fall-back
  2026-11-01 in America/New_York).

- **2026-04-19 — [Stage 3] — The `newSchedule` value is NOT
  persisted to the autosave draft.**
  Rationale: drafts capture the *content* of a future task; the
  operator's notion "I want this picked up 4 hours from now" is
  anchored to wall-clock time, which would silently drift if we
  serialised the absolute UTC instant into the draft and the user
  resumed days later (e.g. "4 hours from now" becoming "4 hours
  ago" after a long weekend). A schedule chosen during a draft
  edit session is reset on close + on draft resume. If draft-side
  scheduling becomes a request, store the chip *kind* + a `now`
  snapshot rather than the absolute instant so the resumed draft
  re-anchors correctly.

- **2026-04-19 — [Stage 3] — `SchedulePicker` takes `appTimezone`
  as a prop instead of calling `useAppTimezone()` internally.**
  Rationale: keeping the picker decoupled from the
  `useAppSettings` hook (which depends on a `QueryClientProvider`
  context) makes it trivially testable with any zone in isolation
  and reusable in stages 4 & 5 where the same picker may render
  inside detail / list contexts that already have the zone in
  scope. Today the create modal looks up the zone once via
  `useAppTimezone()` in `TaskHome.tsx` and forwards it; later
  stages should follow the same pattern.


- **2026-04-19 — [Stage 4] — TaskDetailSchedule renders nothing
  when the task is in a terminal status (`done` / `failed`)
  AND has no schedule.**
  Rationale: terminal tasks never pick up again. Showing a
  "Schedule" button that has no observable effect would be a
  classic dead-affordance UX trap. Edge case: a terminal task that
  *already* carries a schedule (rare — happens when a PATCH flips
  status to `done` while `pickup_not_before` is still set) shows
  a read-only badge with no Edit/Clear controls. Surfacing the
  badge keeps the historical fact visible (operators looking at a
  done task can still tell "this was scheduled for X"), while
  hiding the controls makes it clear the field can no longer be
  meaningfully changed.

- **2026-04-19 — [Stage 4] — TaskDetailSchedule uses a local
  `useMutation` instead of routing through `useTaskPatchFlow`.**
  Rationale: `useTaskPatchFlow` is shaped around the full edit form
  (`title`/`initial_prompt`/`status`/`priority`/`task_type`/`checklist_inherit`)
  and would force the schedule panel to fabricate or thread those
  unrelated fields. The local mutation only sends
  `{ pickup_not_before }` and performs the same query
  invalidations (`taskQueryKeys.all` + `task-stats`) so cache
  refresh behaviour is identical. If `useTaskPatchFlow` ever grows
  a "patch only these fields" mode (or splits per concern), this
  panel can adopt it.

- **2026-04-19 — [Stage 4] — The Edit modal seeds its draft from
  `task.pickup_not_before` only while the modal is closed.**
  Rationale: if a remote PATCH wins via SSE invalidation and the
  underlying task value changes mid-edit, blindly re-seeding the
  draft would clobber the operator's in-progress edit. Re-seeding
  only when the modal isn't open keeps the next "Edit" click
  starting from server truth without stomping on a live editor
  session. Documented because it's the kind of subtle gate where
  the obvious code (always sync) is wrong.

- **2026-04-19 — [Stage 4] — The "Edit" button on an unscheduled
  non-terminal task is labelled "Schedule".**
  Rationale: "Edit" implies an existing value being modified.
  Operators landing on a task that has never been scheduled need
  the affordance to read as "create" semantics, otherwise the
  button feels like dead chrome. Same `data-testid`
  (`task-detail-schedule-edit`) for both states because they're
  the same control wired to the same modal — only the label
  flips.
