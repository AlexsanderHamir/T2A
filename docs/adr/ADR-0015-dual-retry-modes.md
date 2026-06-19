# ADR-0015: Dual Operator Retry Modes After Failure

**Date:** 2026-06-18
**Status:** Accepted
**Deciders:** Engineering

## Context

After a task reaches `failed`, operators need two distinct recovery paths:

1. **Start over** — discard the failed attempt's git/worktree delta and run again from a clean tree.
2. **Resume from failure** — start a new execution cycle that carries forward checkpoint state (verify passes, known commits, feedback) from the failed parent attempt without mutating parent rows or git history.

These must not be conflated with [ADR-0006](ADR-0006-phase-boundary-resume.md) same-cycle resume (`Harness.Resume`), which continues an **open** cycle after process restart while the task is still `running`.

Legacy `PATCH failed→ready` requeued tasks without intent, so the harness could not distinguish fresh vs resume or reset git.

## Decision

Introduce a single pending intent on `tasks.pending_retry` (`{ mode, parent_cycle_id }`), one API (`POST /tasks/{id}/retry`), and one harness entry (`Harness.RunWithRetry`). The worker consumes intent atomically on `ready→running` pickup.

| Mode | Git before cycle | New cycle | Checkpoint |
| --- | --- | --- | --- |
| `fresh` | `git reset --hard` to parent `cycle_base_sha` + `git clean -fd` | `ParentCycleID` + `meta.retry_mode=fresh` | None |
| `resume` | Unchanged | `ParentCycleID` + `meta.retry_mode=resume` | Loaded from parent DB rows |

Authoritative behavior: [retry-start-over.md](../domain/retry-start-over.md), [retry-resume.md](../domain/retry-resume.md).

## Consequences

### Positive

- Operators choose explicitly in the SPA; audit records `task_retry_requested`.
- One column and one harness entry point; no parallel worker dispatch paths.
- Shared checkpoint loader with ADR-0006 resume (`loadVerifyCheckpointData`).

### Negative / Trade-offs

- Fresh retry fails loud when reset anchor is missing (`retry_reset_anchor_missing`) — no silent skip.
- Cross-cycle resume resets `verifyAttempt` to 0 (fresh verify budget per new attempt).
- `PATCH failed→ready` without POST remains legal for scripts but is legacy (no reset, no checkpoint).

## Alternatives Considered

| Alternative | Reason Rejected |
| --- | --- |
| Two columns (`pending_fresh`, `pending_resume`) | Partial state risk; harder to extend |
| Separate harness methods at worker boundary | Duplicates admission and cycle start |
| Reopen terminal cycle row in place | Breaks `attempt_seq` monotonicity and audit |
| Mid-CLI session resume | Runners are stateless; out of scope |

## Related

- [ADR-0006](ADR-0006-phase-boundary-resume.md) — same-cycle resume after restart (unchanged)
- [ADR-0014](ADR-0014-cycle-commit-tracking.md) — `cycle_base_sha` and commit index used by fresh reset anchor chain
