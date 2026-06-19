# ADR-0018: Harness Orchestration State Machine

**Date:** 2026-06-18
**Status:** Accepted
**Deciders:** Engineering

## Context

After ADR-0017 split harness into `internal/*` domain packages, verify retry semantics still lived as inline conditionals in `runCycleLoopVerify`. Contributors extending cycle behavior must read imperative control flow across `cycle_loop.go` rather than a named transition table.

Industry outer-harness patterns (LangGraph FSM, SWE-AF durable steps) separate **pure decisions** from **effect application** (store writes, runner invocation, git subprocesses).

## Decision

Introduce `pkgs/agents/harness/internal/orchestration` as a **pure state machine** package:

| Type | Role |
|------|------|
| `LoopState` | In-memory retry counters and phase position |
| `VerifyResult` | Classify one verify pipeline outcome (pass, retryable fail, tamper) |
| `VerifyEffects` | Retry loop, terminal failure, or tamper flags |
| `DecideVerifyRetry` | Map `(attempt, maxRetries, result)` → effects |
| `VerifyDisabled` | Predicate for legacy checklist-only path |

The harness **root applies effects**: increment `verifyAttempt`, call `terminateCycle`, run `completeChecklistLegacy`, etc. Orchestration imports **domain types only** — no store, runner, or filesystem.

Initial scope covers verify retry/tamper decisions wired from `runCycleLoopVerify`. Execute-phase and loop-level finalize/legacy decisions were added in [ADR-0021](ADR-0021-harness-execute-orchestration.md).

## Consequences

### Positive

- Verify retry budget is table-tested without harness/store setup.
- New terminal paths add rows to `DecideVerifyRetry` rather than nested `if` chains.
- Clear seam for future cycle timeline / lease effects at machine boundaries.

### Negative / Trade-offs

- Split brain until execute transitions migrate — two places to read for full loop semantics.
- DTO duplication between `processState` and `LoopState` until a later unify pass.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Full graph in orchestration day one | Too large for Track B; verify retry is highest-risk branch |
| Keep all logic in cycle_loop | Already at reviewability limits; no testable contract |
| Public `harness/orchestration` package | No external importers; `internal/` enforces boundary |

## Related

- [ADR-0017](ADR-0017-harness-internal-domains.md) — internal package layout
- [docs/domain/harness.md](../domain/harness.md) — durability tiers and cycle lifecycle
