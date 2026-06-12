# ADR-0007: Parent Completion via Done Criteria (Not Subtask Rollup)

**Date:** 2026-06-12
**Status:** Superseded by [ADR-0008](ADR-0008-dependency-satisfies-epic-scheduling.md)
**Deciders:** T2A maintainers

## Context

Subtask scheduling uses `depends_on`: a subtask with `depends_on: [parent_id]` runs only after the parent reaches `status=done`. Separately, `ValidateCanMarkDoneInTx` required **every descendant subtask** to be `done` before the parent could reach `done`.

That pairing deadlocked the common “parent first, then subtasks” workflow:

- Parent could not reach `done` while subtasks were still `ready`.
- Subtasks could not run while the parent was not `done`.

After a successful agent cycle, checklist completions were written but `transitionTask(..., done)` failed silently, leaving the parent stuck in `running` with a succeeded cycle.

## Decision

1. **Parent `done` is criteria-driven.** `ValidateCanMarkDoneInTx` checks only that inherited checklist items are complete (when any exist). It no longer walks the subtask tree.
2. **Root parents with subtasks must define ≥1 done criterion** before subtasks can be linked. Enforced on `POST /tasks` with `parent_id` and when deleting the parent's last owned criterion while subtasks exist.
3. **`depends_on` unchanged.** Subtasks still wait for predecessor `status=done`; once the parent can actually reach `done`, the gate works as designed.

## Consequences

### Positive

- “Wait for parent” + subtasks compose without deadlock.
- Parent completion has an explicit, verify-backed signal when criteria exist.
- Agent harness can transition parent to `done` after verify without waiting for children.

### Negative / Trade-offs

- Parent and subtask completion are independent unless linked by `depends_on`; operators who wanted implicit “rollup” must add explicit dependencies or criteria.
- Root parents with subtasks cannot rely on zero-criteria instant-done; at least one criterion is mandatory.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep subtask rollup, remove `depends_on` parent link | Breaks opt-in “start subtasks after parent completes” scheduling |
| New status (e.g. `parent_complete`) for dependencies | Extra state machine surface; criteria + `done` is sufficient |
| Depend on “criteria complete” without `status=done` | Would change worker readiness; unnecessary once parent can reach `done` |
