# ADR-0008: Dependency Edge Satisfies + Epic Completion Rollup

**Date:** 2026-06-12
**Status:** Accepted
**Supersedes:** [ADR-0007](ADR-0007-parent-completion-via-criteria.md)
**Deciders:** T2A maintainers

## Context

Three independent scheduling questions were conflated into one `status = done` predicate:

1. When may **subtasks** start?
2. When is the **parent epic** finished?
3. Should the **parent agent** re-dequeue mid-epic?

ADR-0007 removed subtask rollup so parents could reach `done` while children were open, unblocking `depends_on: [parent]` at the cost of wrong epic semantics. Future workflow/decision-tree nodes require parent `done` to mean full branch success, while subtasks must start after **verifier-approved criteria** — not after parent `done`.

The verifier already marks criteria via `SetChecklistItemDoneWithEvidence`; checklist completeness is the natural fork signal.

## Decision

1. **Per-edge `task_dependencies.satisfies`:** `done` (default) or `criteria_complete`. Distinct from `task.gate` (manual operator dequeue pause).
2. **`tasks.criteria_satisfied_at`:** denormalized cache maintained in checklist completion TX when `validateChecklistCompleteInTx` transitions; used for SQL queue parity.
3. **Subtask scheduling:** “Wait for parent” → `satisfies: criteria_complete`. Sibling deps stay `done`.
4. **Parent `done`:** checklist complete **and** every descendant `status = done` (restore rollup). Failed/open subtasks block.
5. **Parent self-pickup guard:** after criteria pass with open subtasks, parent is not eligible for agent pickup even if `status = ready`.
6. **Harness:** on successful verify with open subtasks, transition parent `running → ready` (not `done`); checklist/verify path unchanged.
7. **Auto-parent-done:** when the last subtask reaches `done` and parent checklist is complete, cascade parent to `done`.
8. **Wire API:** structured `depends_on: [{ task_id, satisfies }]`. Legacy `string[]` on write maps all edges to `done`.

## Consequences

### Positive

- No deadlock: subtasks start when parent criteria are verified.
- Epic `done` is trustworthy for reporting and future workflow engines.
- Verifier → checklist path is the single fork signal; no parallel “phase complete” mechanism.
- SQL queue and Go readiness share one `edgeSatisfied` semantics via `criteria_satisfied_at`.

### Negative / Trade-offs

- Schema migration + backfill for existing parent dependency edges.
- Breaking change on read: `depends_on` is structured objects, not string ids.
- Parent with subtasks cannot reach `done` until all children succeed.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| ADR-0007 (criteria-only parent done) | Wrong epic semantics; breaks workflow node completion |
| Rollup only, no edge predicate | Deadlock returns |
| New `phase_complete` status | Enum + harness + UI surface area |
| Infer `criteria_complete` when dep == parent_id | Hidden magic |
| Go-only readiness (skip SQL predicate) | Violates queue invariant |
