# Task model (flat hierarchy)

Authoritative semantics for **Project → Task** work items. Wire shapes live in [API-HTTP.md](./API-HTTP.md); this document explains invariants and worker behavior.

## Task shape

| Field | Type | Notes |
|-------|------|--------|
| `tags` | `string[]` | Free-form labels. Each tag: `^[a-z0-9][a-z0-9._-]{0,31}$`. Default `[]`. |
| `milestone` | `string \| null` | Single anchor per task. `^[a-zA-Z0-9][a-zA-Z0-9 ._-]{0,63}$` when set. |
| `parent_id` | `string \| null` | **Depth 1 only**: parent must be a root task (`parent_id IS NULL`). |
| `depends_on` | `string[]` | Hydrated from `task_dependencies`. Directed acyclic graph. |
| `gate` | object \| null | Per-task dequeue pause. `null` = no gate. |

Legacy `project_step_id` remains until goal/step removal (see [proposals/FLAT-TASK-HIERARCHY.md](./proposals/FLAT-TASK-HIERARCHY.md)).

## Dependencies (`depends_on`)

- Storage: `task_dependencies(task_id, depends_on_task_id)` with FK cascade.
- A task in `ready` is worker-eligible only when every predecessor has `status = done`.
- Completing a task notifies dependents whose deps are now satisfied (ready queue).
- Self-deps and cycles → **400** `invalid input`.
- Incremental API: `GET/POST/DELETE /tasks/{id}/dependencies`. Full replace: `depends_on` on `PATCH /tasks/{id}`.

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
- Operator actions: `PATCH /tasks/{id}/gate` with `action` ∈ `release`, `hold`, `clear_hold`.
- V1: auto-release after grace deadline is **not** implemented; release is operator-driven.
- SSE: `task_gate_changed` (id = task id).

## Worker readiness (all must pass)

1. `status = ready`
2. `pickup_not_before` null or ≤ now
3. All `depends_on` predecessors `done`
4. Gate null or `status = released`

If a task is dequeued but fails (3) or (4) on reload, the worker sets `pickup_not_before` ~60s ahead and skips the run.

## Listing filters

Store supports `?tag=` and `?milestone=` on flat list (`ListFlat`). Forest list (`GET /tasks`) is unchanged.

## SSE

| Event | When |
|-------|------|
| `task_dependency_changed` | Dependency add/remove/replace |
| `task_gate_changed` | Gate create/patch/action |

Both invalidate the affected task id in the SPA (see [API-SSE.md](./API-SSE.md)).
