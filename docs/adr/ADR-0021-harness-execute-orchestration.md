# ADR-0021: Harness Execute Orchestration (Decide vs Apply)

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Engineering

## Context

[ADR-0018](ADR-0018-harness-orchestration-fsm.md) introduced `internal/orchestration` for verify retry/tamper decisions (`DecideVerifyRetry`). Execute post-run policy (runner error classification, operator cancel overlay, commit ingest failure) remained imperative in `cycle_loop.go` and `cycle.go`. Verify-disabled legacy and finalize downgrade paths were also inline.

Contributors extending cycle behavior must read two styles: table-driven verify retry vs nested conditionals for execute and loop terminal paths. Terminal writes (`terminateCycle`, `transitionTask`) were duplicated across execute, verify, and finalize branches.

## Decision

Extend `pkgs/agents/harness/internal/orchestration` with execute and loop-level **Decide** functions; consolidate **Apply** in harness root [`cycle_effects.go`](../../pkgs/agents/harness/cycle_effects.go).

| Function | Role |
|----------|------|
| `DecideExecutePostRun` | Runner outcome + cancel + commit ingest → `ExecuteEffects` |
| `DecideVerifyRetry` | Unchanged (verify retry/tamper) |
| `DecideVerifyDisabledLegacy` | Legacy checklist completion err → terminal |
| `DecideFinalizeSuccess` | Completion ledger err → downgrade to failed |

**Boundary rule (unchanged):** orchestration imports `domain` only. Harness maps `runner.Result`/`error` → `ExecuteRunnerOutcome` before Decide.

**Effect applier** owns store ordering: `CompletePhase` before `TerminateCycle`; publish/metrics after successful writes.

**Track C deferred:** unified `Decide(LoopState, Event)` graph; shutdown/panic recovery stays imperative in `recovery.go`.

## Consequences

### Positive

- Execute policy is table-tested without harness/store setup.
- Loop policy (execute, verify retry, verify-disabled, finalize) shares one Decide vs Apply model.
- Terminal paths converge on `applyExecuteEffects` / `applyVerifyEffects` / `applyFinalizeEffects`.

### Negative / Trade-offs

- DTO duplication between `processState` and orchestration inputs until a later unify pass.
- `funclogmeasure` allowlist updates when symbols move.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Import `runner` in orchestration | Breaks leaf purity established in ADR-0018 |
| Execute-only (skip loop-level Decide) | Leaves verify-disabled/finalize imperative |
| Full event graph day one | ADR-0018 rejected; high regression risk |
| Effect applier in orchestration | Would require store/runner imports |

## Related

- [ADR-0017](ADR-0017-harness-internal-domains.md) — internal package layout
- [ADR-0018](ADR-0018-harness-orchestration-fsm.md) — verify retry machine (Track B start)
- [docs/domain/harness.md](../domain/harness.md) — cycle lifecycle reference
