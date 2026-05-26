# Flat task hierarchy

## Feature name

Flatten work hierarchy: **Project → Task** with DAG dependencies, per-task gates, tags, and milestones (replacing Project → Goal → Step → Task → infinite subtasks).

## Problem

T2A today models long-running work as **Project → Goal → Step → Task → recursive subtasks**. Three different mechanisms express “work that depends on other work”:

- Goal-level DAG (`depends_on_goal_ids`)
- Step ordering (`sort_order`) and step gates
- Task `parent_id` (unbounded depth, no task-level `depends_on`)

The agent worker dequeues **tasks** only. Goal/step gates block task **assignment** to a step, not worker dispatch. The product vision is dependency-aware workflows (`Work_Block_1 → Work_Block_2`, parallel blocks, etc.) — that is a **DAG on the unit the worker executes**, not on organizational containers.

This hierarchy adds cognitive and implementation cost without matching how agents pick up work.

## Why this is the next feature

Working backwards from a control plane where agents coordinate long-running workflows:

1. **One unit of work** — The worker runs tasks. Dependencies, gates, and readiness must live on tasks.
2. **Explicit DAG** — `depends_on: [task_ids]` on a join table; worker `ListQueueCandidates` excludes tasks with unresolved predecessors.
3. **Gates where execution pauses** — Optional `gate` JSON on a task; worker skips tasks whose gate is not `released`.
4. **Organization without new tables** — `tags` and `milestone` replace goals/steps as labels; no second gate state machine duplicated at goal and step layers.
5. **Bounded subtasks** — `parent_id` limited to depth 1; deeper decomposition becomes sibling tasks + `depends_on`.

Existing `project_goals` / `project_steps` data is throwaway dev data and will be **wiped** at cleanup (no backfill).

## System impact

### Execution loop

- `ListQueueCandidates` gains predicates: all `task_dependencies` predecessors `done`, and `gate` null or `released`.
- On transition to `done`, store notifies dependents whose deps are now satisfied.
- Worker reloads task after dequeue and re-checks deps/gate before `ready → running`.

### Coordination layer

- Clients model workflows as task graphs under a project, not as parallel goal/step trees.
- SSE: `task_dependency_changed`, `task_gate_changed` (replacing coarse `project_goal_*` / `project_step_*` over time).

### Reliability

- Cycle detection on `task_dependencies` (BFS, same pattern as parent-cycle).
- Additive phases 1–3: old goal/step APIs remain until deprecation/removal.

### Decision quality

- Single contract doc: `docs/TASK-MODEL.md` (authoritative semantics; `API-HTTP.md` links to it).

## Implementation boundary

### In scope

| Concept | Storage | Wire |
|--------|---------|------|
| `depends_on` | `task_dependencies` join table | `Task.depends_on: string[]` |
| `gate` | JSONB on `tasks` | `Task.gate` + `PATCH /tasks/{id}/gate` |
| `tags` | JSONB array on `tasks` | `Task.tags`, `?tag=` filter |
| `milestone` | nullable string on `tasks` | `Task.milestone`, `?milestone=` filter |
| `parent_id` | existing column | depth-1 write rule |

- HTTP: extend task CRUD; dependency endpoints; gate action endpoint.
- Web: task detail panels, create/edit flows; remove goals/steps UI in phase 5.
- Docs: proposal (this file) → `TASK-MODEL.md` → ADR-0002; deprecate then remove goal/step routes.
- Data wipe: `DROP` `project_goals`, `project_steps` at final commit.

### Out of scope (V1)

- Milestone registry table (milestones are free-form strings).
- Auto-release of `gate.pending_release` (operator-driven only).
- Criteria-based gate auto-release (`GateKind` reserved).
- Translating goal-level deps to task deps (data wiped).
- Changes to `ProjectContextItem` / `ProjectContextEdge`.
- Auth, multi-tenant, billing.

## Target model (reference)

```
Project (shared context)
  └── Task
        ├── depends_on: [task_id, ...]
        ├── gate: optional (manual_approval, status, hold, criteria)
        ├── tags: string[]
        ├── milestone: string | null
        └── parent_id: optional (parent must be root task)
```

`TaskCycle` / `TaskCyclePhase` remain the execution substrate per task attempt — orthogonal to project/task planning hierarchy.

## Related docs

- In-flight contract (after C4): [TASK-MODEL.md](../TASK-MODEL.md)
- ADR (after C12): [ADR-0002-flatten-task-hierarchy.md](../adr/ADR-0002-flatten-task-hierarchy.md)
- Execution cycles: [EXECUTION-CYCLES.md](../EXECUTION-CYCLES.md)
- Agent queue predicates: [AGENT-QUEUE.md](../AGENT-QUEUE.md)
