# Data model

Tasks, projects, execution cycles/phases, checklists, dependencies, and gates. HTTP shapes are in [api.md](./api.md); how the worker drives this substrate is in [architecture.md](./architecture.md).

## Project → Task

Work hierarchy is **Project → Task**. Tasks may have:

- `project_id` (optional) — shared-context membership. Projects are long-lived containers for memory across many tasks.
- `tags` and `milestone` — flat labels for organization within a project.
- `depends_on` — directed acyclic graph of task-level dependencies.

`project_id` answers "which long-running body of work shares context with this task?" A project is not a task parent. Multi-step work is modeled as sibling tasks linked by `depends_on`. See [adr/ADR-0010-remove-subtasks.md](./adr/ADR-0010-remove-subtasks.md).

## Task fields

| Field | Type | Notes |
|---|---|---|
| `id` | string (UUID) | Server-assigned when omitted. |
| `title` | string | Required after trim. |
| `initial_prompt` | string (HTML) | TipTap rich text; validated for `@`-mentions when `app_settings.repo_root` is set. |
| `status` | enum | `ready` / `running` / `blocked` / `review` / `done` / `failed` / `on_hold`. Default `ready`. `on_hold` is operator-set: pickup is gated on `status = ready` so an `on_hold` task is intentionally kept out of the worker's queue until the operator flips it back to `ready` (PATCH `/tasks/{id}`). |
| `priority` | enum | `low` / `medium` / `high` / `critical`. Required at create. |
| `project_id` | string \| null | Optional project membership. |
| `project_context_item_ids` | string[] | Explicit allowlist of project context items for runner snapshots. Cleared on `project_id` change. |
| `automation_selections` | object[] | Per-task yes/no toggles for global prompt automations: `{ automation_id, state: yes|no }`. Omit = row absent. Max 20. Injected at harness compose time — see [ADR-0013](./adr/ADR-0013-prompt-automations.md). |
| `tags` | string[] | Free-form, `^[a-z0-9][a-z0-9._-]{0,31}$`. |
| `milestone` | string \| null | Single anchor per task, `^[a-zA-Z0-9][a-zA-Z0-9 ._-]{0,63}$` when set. |
| `depends_on` | object[] | Hydrated from `task_dependencies`: `{ task_id, satisfies }` where `satisfies` is `done` (default and only value). |
| `criteria_satisfied_at` | RFC3339 UTC \| null | Set when all checklist items are verified complete; informational cache on the task row (dependency edges use predecessor `status = done`). |
| `gate` | object \| null | Per-task dequeue pause (see below). |
| `pickup_not_before` | RFC3339 UTC \| null | Defer when the worker may dequeue. |
| `cursor_model` | string | Optional model override at runtime. |

The JSON resource has **no** `created_at` / `updated_at` fields. Timestamps live on `task_events`.

## Dependencies

- Storage: `task_dependencies(task_id, depends_on_task_id, satisfies)` with FK cascade. `satisfies` is `done` (default when omitted).
- A task in `ready` is worker-eligible only when every predecessor has `status = done`.
- Unblocking a predecessor (reach `done`) notifies dependents whose edges are now satisfied.
- Self-deps and cycles return `400 invalid input`.
- API: incremental via `GET/POST/DELETE /tasks/{id}/dependencies`; full replace via `depends_on` on `PATCH /tasks/{id}`. Wire format accepts legacy `string[]` (each id maps to `satisfies: done`) or structured `{ task_id, satisfies }[]`.

Predecessors must reach `status = done`. A predecessor in `failed` or `on_hold` keeps dependents blocked until the operator fixes status or edits dependencies.

## Gate

```json
{
  "kind": "manual_approval",
  "status": "locked | active | pending_release | released",
  "hold": false,
  "pending_release_deadline_utc": "RFC3339 optional",
  "criteria": []
}
```

- Worker dequeue requires `gate IS NULL` OR `gate.status = released`.
- Operator actions: `PATCH /tasks/{id}/gate` with `action ∈ release | hold | clear_hold`.
- Auto-release after grace deadline is **not** implemented; release is operator-driven.

## Worker readiness (all must pass)

1. `status = ready`
2. `pickup_not_before` is null or `<= now()`
3. All `depends_on` predecessors have `status = done`
4. `gate` is null or `gate.status = released`

If a task is dequeued but fails (3) or (4) on reload, the worker sets `pickup_not_before` ~60s ahead and skips the run.

## Scheduling (`pickup_not_before`)

`domain.Task.PickupNotBefore *time.Time` → indexed column `pickup_not_before`. `nil` means "pick up as soon as the worker is free".

- Wire format: RFC3339 UTC string. JSON `null` on `PATCH` clears the field. Empty string is rejected on `POST` (`400`).
- Default deferral on create: `app_settings.agent_pickup_delay_seconds` applies when creating `status=ready` and the client omits `pickup_not_before`.
- Eligibility predicate: `status='ready' AND (pickup_not_before IS NULL OR pickup_not_before <= now())` — see `pkgs/tasks/store/internal/ready/ready.go` (`ListQueueCandidates`). Deep dives: [domain/task-scheduling.md](domain/task-scheduling.md) (four readiness predicates), [domain/agent-queue.md](domain/agent-queue.md) (in-memory queue).
- Three paths to the worker: immediate notify on commit, `PickupWakeScheduler` for future times, reconcile (2m tick) as backstop. **Invariant:** the in-memory queue never contains a task the SQL predicate would reject.
- Single-process: `MemoryQueue` and `PickupWakeScheduler` are not shared across replicas. Keep NTP aligned on app hosts and Postgres so process and DB clocks agree.

## Execution cycles and phases

```text
Task -> many cycles (attempts) -> many phases (steps in an attempt)
```

A **cycle** is one execution attempt. Cycles live in `task_cycles` and are ordered per task by `attempt_seq` (positive integer, `max + 1` assigned by the store inside the same transaction as the insert).

A **phase** is one step inside a cycle. Phases live in `task_cycle_phases` and are ordered per cycle by `phase_seq`. The path is `execute → verify`, with `verify → execute` allowed for corrective retries. A cycle may repeat phase kinds — each visit is a separate row with a higher `phase_seq`.

```mermaid
stateDiagram-v2
  [*] --> execute
  execute --> verify
  verify --> [*]
  verify --> execute : corrective retry
```

`domain.ValidPhaseTransition(prev, next)` defines the graph. Reaching the end of the phase graph is not the same as terminating the cycle — the caller still has to `TerminateCycle(succeeded|failed|aborted)`.

### Schema

```mermaid
erDiagram
  TASKS ||--o{ TASK_CYCLES : "has attempts"
  TASKS ||--o{ TASK_EVENTS : "audit witness"
  TASK_CYCLES ||--o{ TASK_CYCLE_PHASES : "has phases"
  TASK_CYCLES ||--o{ TASK_CYCLE_STREAM_EVENTS : "has stream"
  TASK_CYCLES ||--o| TASK_CYCLES : "parent_cycle_id"
  TASK_CYCLE_PHASES }o--|| TASK_EVENTS : "event_seq backlink"

  TASK_CYCLES {
    string id PK
    string task_id FK
    int64  attempt_seq "unique per task"
    string status      "running|succeeded|failed|aborted"
    time   started_at
    time   ended_at    "null while running"
    string triggered_by "user|agent"
    string parent_cycle_id "optional same-task lineage"
    json   meta_json
  }
  TASK_CYCLE_PHASES {
    string id PK
    string cycle_id FK
    string phase     "execute|verify"
    int64  phase_seq "unique per cycle"
    string status    "running|succeeded|failed|skipped"
    json   details_json
    int64  event_seq "FK-style pointer to most recent mirror row"
  }
```

### Store invariants

- `(task_id, attempt_seq)` and `(cycle_id, phase_seq)` are unique. Stores assign `max + 1` in the same transaction.
- `task_cycles.task_id` and `task_cycle_phases.cycle_id` are FK with `ON DELETE CASCADE`.
- At most one running cycle per task. `StartCycle` rejects with `ErrInvalidInput: task already has a running cycle`.
- At most one running phase per cycle. `StartPhase` rejects with `ErrInvalidInput: cycle already has a running phase`.
- Terminal rows are read-only. Corrective work means a new row with a higher seq.
- Cross-task lineage is rejected. `parent_cycle_id` must reference a cycle on the same task.
- `meta_json` and `details_json` are `jsonb` (Postgres) / `text` (SQLite) and default to `{}`.

### Dual-write invariant

Every cycle/phase mutation appends a mirror row to `task_events` **inside the same `gorm.DB` transaction**. If the mirror append fails, the cycle/phase row is rolled back.

| Store entrypoint | Cycle/phase write | Mirror `task_events.type` |
|---|---|---|
| `StartCycle` | insert `task_cycles` (`status=running`) | `cycle_started` |
| `TerminateCycle(succeeded)` | update to terminal | `cycle_completed` |
| `TerminateCycle(failed|aborted)` | update to terminal | `cycle_failed` (status preserved in payload) |
| `StartPhase` | insert `task_cycle_phases` (`status=running`) | `phase_started` |
| `CompletePhase(succeeded|failed|skipped)` | update to terminal | `phase_completed` / `phase_failed` / `phase_skipped` |

`StartPhase` and `CompletePhase` capture the assigned `task_events.seq` and write it back into `task_cycle_phases.event_seq` in the same transaction. The pointer is one-shot: `CompletePhase` overwrites the `StartPhase` value with the terminal mirror seq.

Mirror rows are non-interactive: `PATCH /tasks/{id}/events/{seq}` returns `400` for these seven types because the cycle/phase row is the system of record.

### Cycle metadata (`meta_json` / `cycle_meta`)

`task_cycles.meta_json` is an adapter-facing sidecar — opaque to the store, contract-defined by the runner. The agent worker writes a stable five-key payload at `StartCycle`:

```json
{
  "runner": "cursor",
  "runner_version": "2.x.y",
  "cursor_model": "",
  "cursor_model_effective": "opus-4",
  "prompt_hash": "sha256:abc123…"
}
```

| Key | Meaning |
|---|---|
| `runner` | `runner.Runner.Name()` at cycle start (e.g. `"cursor"`). |
| `runner_version` | `runner.Runner.Version()` at cycle start. |
| `cursor_model` | Operator intent (verbatim `tasks.cursor_model`). |
| `cursor_model_effective` | Model the runner will actually execute against — audit truth. |
| `prompt_hash` | `sha256` of the prompt string. Never the body. |

Keys are additive only; consumers must ignore unknown keys. Values are always strings (empty string = "no value"). The API surfaces a typed projection `cycle_meta` on `/tasks/{id}/cycles[/{cycleId}]` so the SPA does not re-parse the raw JSON.

### Where reads go

| Question | Read from |
|---|---|
| What's the current attempt for this task? | `task_cycles` (`status=running`, latest `attempt_seq`). |
| List all attempts for this task. | `GET /tasks/{id}/cycles`. |
| What phase is the current cycle in? | `GET /tasks/{id}/cycles/{cycleId}` (`phases[]`, `phase_seq ASC`). |
| Audit history (everything that happened, in order). | `GET /tasks/{id}/events`. |
| Cursor live-update history for one attempt. | `GET /tasks/{id}/cycles/{cycleId}/stream`. |
| Did anything change for this cycle (live UI hint)? | SSE `task_cycle_changed` (`id` = task, `cycle_id` = cycle). |

## Checklist (done criteria)

Behavioral deep-dives: [domain/harness.md](./domain/harness.md) (orchestration), [domain/done-criteria.md](./domain/done-criteria.md) (full lifecycle), [domain/execute-agent.md](./domain/execute-agent.md) (execute phase), [domain/verify-agent.md](./domain/verify-agent.md) (verify phase), [domain/project-context.md](./domain/project-context.md) (context snapshots), [domain/persistence.md](./domain/persistence.md) (dual-write), [domain/task-events.md](./domain/task-events.md) (audit log).

Per-task acceptance requirements. Stored in `task_checklist_items` (definitions: `id`, `task_id`, `sort_order`, `text`) and optional `task_checklist_item_commands` (per-criterion shell checks: `item_id`, `sort_order`, `command`, `expected_outcome`, `ON DELETE CASCADE`) and `task_checklist_completions` (per-subject ledger: `task_id`, `item_id`, `at`, `done_by`, `evidence`, `verified_by`, `verifier_reasoning`, `cycle_id`). Operators attach zero or more verification commands per criterion; during verify the worker runs them in `app_settings.repo_root`, writes stdout/stderr/meta under the worker-managed report dir, and feeds those artifacts to the verify agent. The LLM remains the sole authority for marking criteria done — exit code 0 does not auto-pass.

**Create:** `POST /tasks` requires at least one non-empty done criterion in `checklist_items`; definition rows are inserted in the same transaction as the task row.

**Completion:** Marking a task `done` requires its checklist to be complete when criteria exist. Execution order among related tasks is expressed only via `depends_on` (see Dependencies).

**Definition edits:** Operators may add checklist definition rows only while the task is not `running` or `done`. Once the agent has picked up the task, criteria are locked; edit and delete remain subject to the existing cycle-running and verified-item guards documented in [api.md](./api.md).

| `verified_by` value | Meaning |
|---|---|
| `agent_self` | Failure-only: execute did not claim done in the criteria report. Never written on success. |
| `verify_agent` | Verify phase accepted the criterion (sole authority for marking done). |
| `deterministic_check` | Legacy rows only (pre–verify-only migration); never written by the current worker. |
| `human_override` | Reserved; schema only. |
| `legacy` | Pre-V1.1 rows backfilled at migrate; never written by the new worker. |

### Edit locks

| State | Add | Edit text | Delete | Agent mark done |
|---|---|---|---|---|
| Open (no running cycle) | yes | yes | yes* | yes |
| Cycle running | no (409) | no (409) | no (409) | yes |
| Verified (completion exists) | yes | no (409) | no (409) | yes |

\*Delete blocked if any subject has marked the item done.

### Worker verification loop

Verify runs after every successful execute **only when the task has at least one criterion**. Tasks with **zero criteria** (legacy rows created before the create-time requirement) skip verify and write no checklist completion rows — a successful execute alone marks the task `done`.

1. **Execute** — prompt prepends all criteria with stable ids and the **absolute** worker-managed path the agent must write its report to (`<worker-managed dir>/<cycle_id>/criteria-report.json`, see "Report file contracts" below). `claimed_done` in the report is an assertion only — not final acceptance.
2. **Gate** — criteria with `claimed_done: false` fail immediately (`verified_by=agent_self`); no verify pass for those ids.
3. **Verify** — for every criterion that passed the gate, the verify runner runs in the execute working dir (where execute's uncommitted changes live so the verifier can inspect actual file contents) and writes its verdict to the **absolute** worker-managed `<worker-managed dir>/<cycle_id>/verify-report.json` path. The verifier MUST NOT modify any path inside the working dir. The worker enforces this with a pre/post integrity snapshot of `git status --porcelain` plus `git rev-parse HEAD`; the whitelist is empty (report files live outside the working tree, so any porcelain diff is tampering), any HEAD movement, or any failure to capture the post-snapshot terminates the cycle as `verify_tampered` (terminal — no retries, no completion rows). When the working dir is not a git repo, the integrity check is bypassed and logged once at startup. Adversarial separation: when `app_settings.verify_runner_name` is set, the verify pass runs on a different runner adapter (and optionally a different model) than execute — see `docs/configuration.md`. Successful criteria are marked `verified_by=verify_agent`. On `CompletePhase`, verify phases also write a structured `details.verification` snapshot (attempt seq, pass/fail counts, per-criterion text/reasoning) into the mirrored `phase_completed` / `phase_failed` audit event so the SPA audit timeline and event detail page can explain outcomes without a verdicts round-trip.
4. **Decision** — all pass → atomic `SetDoneWithEvidence` + `status=done`; any fail → retry execute up to `verify_max_retries` or terminate with reason `verification_failed:<id>,<id>,…` (sorted, deduped failing criterion IDs after the prefix) and **no** completion rows. The `verification_failed` prefix is contract-stable; consumers MUST use prefix matching (`startsWith`). Bare `verification_failed` (older cycles) remains a valid value. The reason column is 256 chars; long failure lists are truncated with a trailing `…` while keeping the prefix intact.
5. **Retry efficiency** — verdicts that passed in earlier attempts are carried in memory across retries. The next execute prompt lists them under "Already verified (do not re-do)" and excludes them from the active checklist; the next verify pass short-circuits them. The atomic-decision contract is preserved: nothing is committed to `task_checklist_completions` until the cycle terminates `succeeded`, at which point all passes (this attempt + earlier) land in one transaction. On terminal failure, no completion rows are written even for criteria that passed on every attempt.

Pre-V1.1 completion rows may carry `verified_by=legacy` (backfilled at migrate); the current worker never writes that value.

### Report file contracts

Paths live under a **worker-managed scratch directory** (`<worker-managed dir>/<cycle_id>/...`) which the operator never sees. The worker resolves the directory from `T2A_WORKER_REPORT_DIR` (default `<os.TempDir()>/t2a-worker`); the agent CLI is told the absolute path in its prompt and writes there directly. The directory lives outside `app_settings.repo_root` so customer working trees stay clean and the verify-pass integrity check has an empty whitelist (any porcelain diff against the working tree during verify is tampering). The per-cycle subdirectory is GC'd by the worker at cycle terminate so disk use stays bounded.

| File | Writer | Schema |
|---|---|---|
| `<worker-managed dir>/<cycle_id>/criteria-report.json` | Execute agent | `{ "criteria": [{ "id", "claimed_done", "evidence" }], "commits": [{ "sha", "branch" }] }` — `commits` optional; worker validates SHAs against git ancestry at execute ingest ([ADR-0014](adr/ADR-0014-cycle-commit-tracking.md), [domain/cycle-commits.md](domain/cycle-commits.md)) |
| `<worker-managed dir>/<cycle_id>/verify-report.json` | Verify agent | `{ "criteria": [{ "id", "verified", "reasoning" }] }` |

Limits: 256 KB per report file; `evidence` and `reasoning` ≤ 16 KB each; verify `reasoning` ≥ 40 chars when `verified=true`. Duplicate ids in a report → invalid. Symlinks rejected.

### Verdict tables

The two report files above are the agent ↔ worker wire format. They are GC'd at cycle terminate, so they are NOT the durable record. The worker mirrors each parsed report into one row per criterion per attempt in two normalized tables. These rows are the source of truth for the SPA's verdict UI and for support / prompt-tuning analytics.

`task_cycle_criteria_reports`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid pk | server-assigned at upsert. |
| `cycle_id` | string fk → `task_cycles.id` (`ON DELETE CASCADE`) | indexed; verdicts disappear with their cycle. |
| `attempt_seq` | int (>0) | which retry attempt produced this row. |
| `criterion_id` | string fk → `task_checklist_items.id` (`ON DELETE NO ACTION`) | non-cascade preserves verdict history if an operator deletes a criterion later. |
| `claimed_done` | bool | execute agent's self-claim from `criteria-report.json`. |
| `evidence` | text (≤ 16 KB at the boundary) | mirrored from the file. |
| `written_at` | timestamptz | indexed. |

`task_cycle_verify_reports`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid pk | server-assigned. |
| `cycle_id` | string fk → `task_cycles.id` (`ON DELETE CASCADE`) | indexed. |
| `attempt_seq` | int (>0) | retry attempt. |
| `criterion_id` | string fk → `task_checklist_items.id` (`ON DELETE NO ACTION`) | history preserved on criterion delete. |
| `verified` | bool | per-criterion verdict for this attempt. |
| `verifier_kind` | string | one of `domain.VerifierKind` (`agent_self` / `verify_agent` / `deterministic_check` / `human_override` / `legacy`); same enum as `task_checklist_completions.verified_by` so the SPA can render the same chip in both surfaces. |
| `reasoning` | text (≤ 16 KB) | verifier rationale. |
| `written_at` | timestamptz | indexed. |

`task_cycle_commits`

Worker-indexed git commits for one cycle ([ADR-0014](adr/ADR-0014-cycle-commit-tracking.md)). Upserted after a successful execute run (before `CompletePhase(execute)`) from ancestry `cycle_base_sha..HEAD`. Not dual-written to `task_events`.

| Column | Type | Notes |
|---|---|---|
| `id` | uuid pk | server-assigned at upsert. |
| `task_id` | string fk → `tasks.id` (`ON DELETE CASCADE`) | denormalized for list-by-task. |
| `cycle_id` | string fk → `task_cycles.id` (`ON DELETE CASCADE`) | indexed. |
| `phase_seq` | int (>0) | execute phase that last ingested this SHA. |
| `seq` | int (>0) | order within cycle ancestry (`1..N`); unique index with `cycle_id` for ordering. |
| `repo` | text | `app_settings.repo_root` at snapshot. |
| `worktree` | text | `git rev-parse --show-toplevel`. |
| `branch` | string | from agent report, `git branch --contains`, or snapshot `base_branch`. |
| `sha` | string | full commit hash; unique with `cycle_id`. |
| `committed_at` | timestamptz | from `git log -1`. |
| `message` | text | subject line from `git log -1`. |
| `recorded_at` | timestamptz | worker upsert time. |

Unique index: `(cycle_id, sha)`. List order: `seq ASC`. Pre-ADR-0014 cycles return zero rows.

`task_cycle_command_runs`

| Column | Type | Notes |
|---|---|---|
| `id` | uuid pk | server-assigned. |
| `cycle_id` | string fk → `task_cycles.id` (`ON DELETE CASCADE`) | indexed. |
| `attempt_seq` | int (>0) | verify attempt that ran the command. |
| `criterion_id` | string fk → `task_checklist_items.id` (`ON DELETE NO ACTION`) | which criterion owned the command. |
| `command_seq` | int (≥0) | `sort_order` from `task_checklist_item_commands`. |
| `exit_code` | int | process exit code (`-1` when start/wait failed). |
| `meta_path` | text | absolute path to `<report_dir>/<cycle_id>/checks/<criterion_id>/<seq>.meta.json`. |
| `written_at` | timestamptz | indexed. |

Stdout/stderr bytes live only in temp files under the worker report dir (see `<worker-managed dir>/<cycle_id>/checks/...`); this table is the durable audit index for the SPA timeline.

Both verdict tables enforce a composite unique index on `(cycle_id, attempt_seq, criterion_id)`. Command runs enforce `(cycle_id, attempt_seq, criterion_id, command_seq)`.

Pre-PR2 cycles return zero rows from these tables; the handler returns empty arrays, never 404. Cleanup is FK-driven: deleting a cycle (which itself cascades from task deletion) cascades to the verdict rows; `criterion_id` is intentionally `NO ACTION` so that historical cycles remain readable after a checklist edit.

## Prompt automations

Global reusable behavioral instructions stored in `automations` (`id`, `title`, `description`, timestamps, optional `archived_at` soft delete). Not scoped to a project.

Tasks bind toggles in `automation_selections` JSONB (max 20): `{ automation_id, state: yes|no }`. **Omit** means the automation id is absent from the array. The harness resolves library text at compose time and injects a `## Agent behaviors` block into the execute prompt (see [ADR-0013](./adr/ADR-0013-prompt-automations.md)).

The SPA exposes library CRUD on `/automations` and Yes / Omit / No pickers in the create-task modal. `GET /v1/bootstrap` seeds the automation catalog for cold start.

## Project context

Curated context nodes (`project_context_items`) and user-curated relationships (`project_context_edges`, typed `relation` + `1..5 strength`) owned by a project. A task's run captures the user-selected bundle in `task_context_snapshots` — immutable, cycle-scoped.

Mental model: project = process, task = thread. Project = shared memory; task = reader; run = immutable snapshot of what the runner actually saw.

Out of scope today: embeddings / vector search, autonomous memory pruning, summarization daemons, tenancy / sharing / billing, automatic migration of legacy tasks into synthetic projects.

## Audit log (`task_events`)

Append-only. Event type strings are `domain.EventType` values (`task_created`, `status_changed`, `prompt_appended`, `message_added`, checklist events, etc., plus the seven cycle/phase mirror types listed above). Per-task monotonic `seq`. Used for history and debugging; events are not replayed into the SSE hub.
