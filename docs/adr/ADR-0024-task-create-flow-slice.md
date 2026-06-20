# ADR-0024: Task Create Flow Vertical Slice (Frontend Decide vs Apply)

> **Superseded in part (2026-06-20):** Draft task evaluation (create-modal Evaluate button, `POST /tasks/evaluate`, invariant I5) was removed. Historical invariant and race-test references below remain for context.

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Engineering

## Context

[`useTaskCreateFlow.ts`](../../web/src/tasks/hooks/useTaskCreateFlow.ts) (~1,238 lines) owned form state, modal lifecycle, draft autosave, evaluate/create mutations, draft list queries, and a 70-field flat return API. Race contracts (stale create/evaluate/autosave when switching drafts) were enforced via ref checks inside mutation `onSuccess` handlers, with integration coverage in [`useTasksApp.test.tsx`](../../web/src/tasks/hooks/useTasksApp.test.tsx).

[ADR-0022](ADR-0022-task-sync-policy.md) established Decide → Apply for SSE cache policy; [ADR-0023](ADR-0023-task-scheduling-domain.md) did the same for backend readiness. Create flow is the largest remaining frontend cohesion debt.

## Decision

Introduce **`web/src/tasks/create/`** as the owned vertical slice:

| Layer | Modules |
|-------|---------|
| Pure Decide | `validateCreateForm`, `draftPayload`, `buildCreateMutationInput`, `decideCreateEntry`, `mapCreateFlowViewModel` |
| Apply (hooks) | form/modal state, mutations, autosave, entry/submit/checklist actions, composer |
| Shim | [`hooks/useTaskCreateFlow.ts`](../../web/src/tasks/hooks/useTaskCreateFlow.ts) re-exports public API unchanged |

UI stays in [`task-create-modal/`](../../web/src/tasks/components/task-create-modal/) for V1.

### Invariants (I1–I7)

| ID | Invariant |
|----|-----------|
| I1 | No autosave while `editingTaskId != null` |
| I2 | Autosave baseline / label updates only when `saved.id === newDraftIDRef.current` |
| I3 | Modal closes on create success only when `variables.draft_id === newDraftIDRef.current` |
| I4 | Resume last-wins via `requestedResumeRef` |
| I5 | Evaluation snapshot applies only when `variables.id === newDraftIDRef.current` |
| I6 | Default `project_id` sent on create when dropdown unchanged |
| I7 | Entry routing: loading → picker; error → fresh form + hint; drafts → picker; else fresh form |

### Boundary rules

- `create/*.ts` (non-hooks): no `react` imports; no `task-create-modal` imports
- `create/hooks/*`: may import React, `@/api`, create pure modules
- `hooks/useTaskCreateFlow.ts`: re-export shim only

## Consequences

### Positive

- Pure entry/validation/draft payload rules are unit-testable without hooks
- Hook files split by responsibility; file-size bar restored
- Stable flat API for `useTasksApp` during migration

### Negative / Trade-offs

- Temporary shim at old import path
- `applyResumedDraftToForm` still uses setter callbacks (Apply); pure field mapper is a follow-up

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Move modal tree under `create/components/` | Large CSS/import churn; defer |
| Nested return type (`createFlow.form.*`) | Breaks `useTasksApp` destructuring |
| React Context | Wide re-renders; composition sufficient |
| Extend `sync/` mutation guard to create | Out of scope per ADR-0022 |

## Related

- [docs/web.md](../web.md) — task create read order
- [web/src/tasks/create/README.md](../../web/src/tasks/create/README.md)
