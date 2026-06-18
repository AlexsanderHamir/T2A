# ADR-0014: Cycle commit tracking

**Date:** 2026-06-18
**Status:** Accepted
**Deciders:** Engineering
**Supersedes:** ADR-0006 execute commit marker policy (partial)

## Context

Execute-phase prompts required agents to embed `t2a:cycle=<cycle_id>` in commit
messages so resume and verify could grep for cycle-scoped work. The worker never
persisted SHAs, never enforced a clean tree, and leaked internal IDs into public
git history.

ADR-0006 added phase-boundary resume using phase ledger + verdict tables + optional
grep fallback. That fallback fails when the tree is clean but commits lack markers.

## Decision

1. **No public ID markers** — remove `t2a:cycle=` from all prompts; use normal
   commit messages only.
2. **Always-on commits in git repos** — delete `app_settings.agent_commit_execute_work`;
   execute must end with commits + clean tree before verify (non-git repos skip).
3. **`task_cycle_commits` table** — durable index per cycle: repo → worktree →
   branch → commit (mirrors ADR-0004 verdict pattern).
4. **Ancestry discovery** — at execute start snapshot `base_sha` and
   `cycle_base_sha` (first execute in cycle); at ingest run
   `git rev-list --reverse cycle_base_sha..HEAD`.
5. **Upsert semantics** — unique `(cycle_id, sha)`; no delete-on-retry. Agents
   create **new commits only** (no amend/rebase of cycle work).
6. **Ingest timing** — after runner success, before `CompletePhase(execute)`.
7. **Consumers** — verify/resume prompts and `GET .../verdicts` read from DB;
   live `git diff HEAD` retained for uncommitted remainder.

## Consequences

### Positive

- Verify and resume get worker-owned commit lists without grep.
- Execute gates block empty ancestry and dirty trees.
- UI shows per-cycle commit timeline via verdicts API.

### Negative / Trade-offs

- Correctness depends on agent discipline (new commits only); rewrites fail
  `execute_rewritten_history`.
- Single shared `repo_root` still allows concurrent human commits in range.
- Criteria report mirror still lands at verify entry (follow-up ADR-0015 candidate).

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep message markers | Pollutes public git history |
| Replace-all commits on retry | Loses stable SHAs when only appending |
| Store diff blobs | Size, redaction, duplication of git |

## Related

- [ADR-0004](ADR-0004-verdicts-on-the-db.md) — durable mirror pattern
- [ADR-0006](ADR-0006-phase-boundary-resume.md) — resume model unchanged; marker policy superseded
- [docs/domain/cycle-commits.md](../domain/cycle-commits.md)
