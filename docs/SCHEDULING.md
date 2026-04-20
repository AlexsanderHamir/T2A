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
