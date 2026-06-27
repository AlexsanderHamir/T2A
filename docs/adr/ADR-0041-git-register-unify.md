# ADR-0041: Unified git repository registration

**Date:** 2026-06-27  
**Status:** Accepted  
**Deciders:** Engineering (git/worktrees vertical)

## Context

Hamix had two repository registration paths:

- `CreateGlobalGitRepository` — resolved `git_common_dir`, deduped on common dir, repo row only.
- `CreateGitRepository` — opened path only, deduped on `path`, seeded main worktree and **all** local branches.

That divergence caused missing `git_common_dir` on project-scoped creates, path-based duplicate detection, and a full branch catalog in Postgres that git already owns.

## Decision

1. **Single internal entrypoint** — `registerGitRepository` in `pkgs/tasks/store/store_git_register.go`:
   - Always `ResolveRegistration` → set `path` + `git_common_dir`.
   - Dedupe on `git_common_dir` only.
2. **Project-scoped create** delegates to `registerGitRepository`, then `seedMainWorktreeWithCurrentBranch` (one branch row for the checkout at main root).
3. **Path compare** uses `gitwork.PathKey` / `PathKeyEqual` everywhere, including `BelongsToRepository`.
4. **Reconcile** matches linked worktrees path-first, then by bound branch name; reports `branch_checkout_mismatch` when live checkout ≠ binding.
5. **Repair pipeline** runs `git worktree repair` then `git worktree prune` when `RepairGit` is set.
6. **Live inventory API** exposes `locked` and `prunable` from porcelain.

## Consequences

### Positive

- One identity model (`git_common_dir`) for all registration routes.
- Smaller branch table — only bound branches plus operator-created rows.
- Reconcile survives path moves when path key still matches; branch renames surface explicit partial status.

### Negative / Trade-offs

- `git_repositories.path` unique index remains (deferred migration).
- `head_sha` column remains a cache (reconcile still refreshes bound rows).
- Branch catalog for modals uses `/branches/live`, not DB list alone.

## Alternatives Considered

| Alternative | Reason Rejected |
|-------------|-----------------|
| Keep dual registration paths | Continued drift and duplicate logic |
| Drop all branch rows; derive only from git | Breaks immutable `branch_id` FK on worktrees |
| Reconcile by branch name only | Silent failure on branch rename |

## See also

- [ADR-0040](./ADR-0040-git-reconcile-v2.md)
- [docs/domain/worktrees-and-branches.md](../domain/worktrees-and-branches.md)
