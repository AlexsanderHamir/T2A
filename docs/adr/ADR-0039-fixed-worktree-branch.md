# ADR-0039: Fixed branch per worktree, task binds worktree_id

**Date:** 2026-06-27
**Status:** Accepted (supersedes ADR-0037 §2–4 on multi-association and checkout switching)
**Deciders:** Hamix maintainers

## Context

ADR-0037 modeled a worktree as a directory that could associate with **many** repo-level branches over time via `worktree_branches`, with the worker running `git checkout` before each agent run. Operators and product rules changed:

- **One branch per worktree**, chosen at register/create time and immutable thereafter.
- **No branch switching** at task pickup — the worktree must already be on its branch.
- **Tasks bind `worktree_id`**; branch is derived from `git_worktrees.branch_id`.
- **Execution:** sequential within a worktree, parallel across worktrees (in-process worker pool).

## Decision

### 1. Collapse the git chain

`repo → worktree(path + branch_id) → task(worktree_id)`.

Drop table `worktree_branches`, column `tasks.worktree_branch_id`, column `git_worktrees.active_branch_id`.

### 2. Invariants

- Each worktree has exactly one `branch_id` (NOT NULL after migration).
- Each branch is bound to at most one worktree (`UNIQUE(branch_id)` on `git_worktrees`).
- Worker sets `WorkingDir` from worktree path; optional HEAD verify fails on drift.
- Per-worktree gate serializes runs on the same directory; worker pool allows different worktrees concurrently.

### 3. API

- Remove `GET/POST/DELETE /git/worktrees/{id}/branches`.
- Task create/patch accept `worktree_id` (not `worktree_branch_id`).

## Consequences

### Positive

- Model matches operator mental model and git layout.
- Removes checkout switching, active-branch tracking, and association CRUD.
- Simpler validation and worker git prep.

### Negative / trade-offs

- Data migration required for existing `worktree_branches` rows.
- Changing a worktree branch requires delete + recreate (no in-place mutation).

## See also

- [ADR-0037](./ADR-0037-global-repos-project-tree.md)
- [docs/domain/worktrees-and-branches.md](../domain/worktrees-and-branches.md)
