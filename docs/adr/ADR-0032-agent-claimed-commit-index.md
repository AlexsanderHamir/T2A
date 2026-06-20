# ADR-0032: Agent-claimed commit index

**Date:** 2026-06-19
**Status:** Accepted
**Deciders:** Operator + harness contributors
**Supersedes:** [ADR-0016](./ADR-0016-observe-vs-admit-commits.md)

## Context

ADR-0016 indexed commits via `git rev-list cycle_base_sha..HEAD` and gated execute on hygiene (no commits, dirty tree, rewritten history). Status columns (`eligible`, `observed`, `inherited`, `superseded`) filtered what verify could see.

Operators want:
1. Agents to declare commits explicitly in `criteria-report.json`.
2. Additive-only git policy enforced in prompts + verify, not execute gates.
3. A task-wide append-only ledger for verify across cycles/attempts.

## Decision

1. **Ingest source:** `commits[]` in `criteria-report.json` → `cat-file` + `git log -1` → `UpsertCycleCommits`.
2. **No execute hygiene gates** — empty claims, dirty tree, and rewritten history do not set `FailReason` on ingest.
3. **Schema:** remove `status`, `gate_reason`, `source_cycle_id` from `task_cycle_commits` and API JSON.
4. **Verify read:** `ListCommitsForTask(task_id)` — all unique SHAs ever indexed for the task.
5. **Append-only:** no `MarkCycleCommitsSuperseded`; rows are never deleted on re-ingest.

## Consequences

### Positive

- Verify-only child cycles see parent commits via task-wide ledger.
- Simpler API and UI (no status badges).
- Agent contract is explicit and testable.

### Negative / Trade-offs

- Incomplete `commits[]` from the agent leaves holes in the index (mitigated by prompts).
- Historical DB rows may reference SHAs removed by amend/rebase (optional warn on ingest).

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep rev-list as fallback indexer | Two sources of truth; agent claims are the contract |
| Keep observe/admit statuses | Complexity without execute gates |

## See also

- [docs/domain/cycle-commits.md](../domain/cycle-commits.md)
