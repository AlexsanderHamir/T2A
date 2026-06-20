# Task create flow (`web/src/tasks/create/`)

Vertical slice for create-task modal policy, draft autosave, and mutations. UI lives in [`../components/task-create-modal/`](../components/task-create-modal/) for V1.

## Read order

1. [ADR-0024](../../../docs/adr/ADR-0024-task-create-flow-slice.md) — invariants and boundaries
2. Pure Decide modules (`validateCreateForm`, `draftPayload`, `decideCreateEntry`, `buildCreateMutationInput`, `mapCreateFlowViewModel`)
3. Hooks under `hooks/` — Apply layer composed by `useTaskCreateFlow`
4. Public API: [`index.ts`](index.ts) (shim at [`../hooks/useTaskCreateFlow.ts`](../hooks/useTaskCreateFlow.ts))

## Invariants (I1–I4, I6–I7)

| ID | Rule | Where enforced |
|----|------|----------------|
| I1 | No autosave while editing | `useTaskCreateDraftAutosave` |
| I2 | Baseline stamp only for active draft | `useTaskCreateMutations` save `onSuccess` |
| I3 | Close modal on create only for active draft | `useTaskCreateMutations` create `onSuccess` |
| I4 | Resume last-wins | `useTaskCreateEntryActions` + `requestedResumeRef` |
| I6 | Default project on unchanged dropdown | `buildCreateMutationInput` |
| I7 | Entry routing (loading/error/drafts/fresh) | `decideCreateEntry` + `useTaskCreateEntryActions` |

Race integration tests: [`../hooks/useTasksApp.test.tsx`](../hooks/useTasksApp.test.tsx).

## Debug checklist

- **Stale "Draft saved" after switching drafts:** check `newDraftIDRef` vs save response `id` (I2).
- **Modal closed but wrong task created:** check create `variables.draft_id` vs ref (I3).
- **Wrong draft fields after rapid resume clicks:** check `requestedResumeRef` (I4).
