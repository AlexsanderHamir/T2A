# ADR-0016: Observe-vs-admit commit statuses

**Date:** 2026-06-18
**Status:** Superseded by [ADR-0032](./ADR-0032-agent-claimed-commit-index.md)
**Deciders:** Engineering
**Supersedes:** ADR-0014 ingest timing (partial — observation always persists)

## Context

ADR-0014 indexed git commits only when every execute admission gate passed
(clean tree, no rewrite, non-empty ancestry). When the runner exited OK but a
gate failed (`execute_uncommitted_work`, `execute_invalid_commit`), **zero**
rows were written. Cross-cycle resume then had empty `knownCommits`, so agents
re-ran discovery-style work and picked different targets on each attempt.

Operators also had no UI visibility into commits from failed execute attempts.

## Decision

1. **Observe-first ingest** — after a successful runner exit, always upsert
   commits discovered via `git rev-list cycle_base_sha..HEAD` before evaluating
   admission gates.
2. **Commit status enum** — `eligible`, `observed`, `inherited`, `superseded`
   on `task_cycle_commits` with optional `gate_reason` and `source_cycle_id`.
3. **Admission unchanged** — gates still fail execute when dirty tree, rewrite,
   or parse errors; failed attempts terminate with the same machine reasons.
4. **Verify reads eligible only** — `ListEligibleCommitsForCycle`; observed
   commits are resume context, not verify-ready evidence.
5. **Inherited promotion** — commits copied from a parent attempt on the
   zero-new-commit resume path start as `inherited` and promote to `eligible`
   when gates pass on re-admission.
6. **Supersede on rewrite** — SHAs dropped from current ancestry are marked
   `superseded` rather than deleted.
7. **Continuation bundle** — cross-cycle resume loads one struct classifying
   parent failure, scope files, commits by status, and routes verify-only when
   parent execute succeeded with eligible commits.

Existing rows backfill to `eligible` (they only existed under the old
all-or-nothing path).

## Consequences

### Positive

- Failed execute attempts preserve SHAs for resume and UI.
- Verify cannot advance on observed-only commits.
- Task-wide commit panel can show status chips and gate reasons.

### Negative / Trade-offs

- More rows per failed attempt; dedupe by SHA prefers highest status rank.
- Verify-only cross-cycle resume requires eligible commits on the parent cycle.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep all-or-nothing ingest | Resume loses commit context on gate failure |
| Delete observed rows on retry | Loses audit trail; breaks task-wide index |
| Verify on observed commits | Violates admission contract |

## Related

- [ADR-0014](ADR-0014-cycle-commit-tracking.md) — base commit index
- [ADR-0015](ADR-0015-dual-retry-modes.md) — resume vs fresh retry
- [docs/domain/commit-eligibility.md](../domain/commit-eligibility.md)
- [docs/domain/resume-continuation.md](../domain/resume-continuation.md)
