# ADR-0033: Git worktrees and branches (Issue #39)

**Date:** 2026-06-22  
**Status:** Accepted  
**Deciders:** Hamix maintainers

## Context

Hamix v0 shipped with a single global `app_settings.repo_root`: one working directory for every agent run, `/repo/*` autocomplete, and `@`-mention validation. [Issue #39](https://github.com/AlexsanderHamir/Hamix/issues/39) requires tasks bound to a **worktree + branch**, UI CRUD for worktrees/branches, and Docker host integration.

We evaluated [Worktrunk](https://github.com/max-sixty/worktrunk) as an external worktree CLI. Hamix needs **DB authority** (tasks, delete guards, queue, SPA) and verify-in-place semantics ([ADR-0003](./ADR-0003-verify-component-upgrade.md)); Worktrunk is terminal-first with no Hamix task model.

## Decision

1. **Native `git` backend** — `pkgs/gitwork/` wraps `git worktree`, `git branch`, and `git checkout` via subprocess (not Worktrunk in v0.1).
2. **Entities** — per project (default project in UI):
   - `git_repositories` — registered main checkout (`path`, optional `host_path`, `default_branch`)
   - `git_worktrees` — linked working directories (`repository_id`, `path`, `name`, `is_main`)
   - `git_branches` — tracked branches (`repository_id`, `name`, `head_sha`)
3. **Task binding** — `tasks.worktree_id` and `tasks.branch_id` required on create; multiple `ready` tasks may queue on the same pair; worker mutex serializes execution per worktree.
4. **Worker** — `WorkingDir` = task worktree path; `gitwork.Checkout` for bound branch before harness; supervisor idle when `no_repository_registered` or `all_worktrees_invalid` (not `repo_root`).
5. **Delete guards** — `DELETE` worktree/branch returns **409** `has_running_task` when any task on the target has `status=running`; queued tasks do **not** block delete.
6. **Scoped repo API** — `/repo/*` requires `?worktree_id=`; `@`-mentions validate against that worktree on task create/patch.
7. **Verify-in-place preserved** — verify runs in the same cwd as execute; Hamix controls checkout timing.
8. **Cursor session resume** — resume key is worktree path + branch ([ADR-0031](./ADR-0031-cursor-session-resume-default.md)); workspace mismatch deny-list still applies.
9. **Docker** — container-canonical paths in DB; `HAMIX_PATH_MAP` translates for SPA display; `git` in image; host repos via bind mounts ([docs/docker.md](../docker.md)).
10. **Deprecation** — `app_settings.repo_root` removed after backfill migration; `on_task_done` audit event carries `worktree_id`, `branch_id`, `commits[]` for future PR automation (no UI in v0.1).

## Consequences

### Positive

- Multiple parallel work contexts on one host without a global settings path.
- Delete semantics and task binding are enforceable in SQL.
- Production Docker path mapping is explicit.

### Negative / trade-offs

- DB rows must reconcile with `git worktree list` (reconcile endpoint).
- Sequential worker retained — no multi-worktree parallel runs in v0.1.
- Worktrunk operator ergonomics deferred.

## Alternatives considered

| Alternative | Reason rejected |
| --- | --- |
| Worktrunk as primary backend | CLI-only; dual config surface; no Hamix task model |
| Keep `repo_root` forever | Conflicts with per-task binding and Docker path model |
| Block delete when queued tasks exist | Issue #39 allows delete with only **running** guard |

## See also

- [docs/plans/issue-39-git-workflow-roadmap.md](../plans/issue-39-git-workflow-roadmap.md)
- [docs/domain/worktrees-and-branches.md](../domain/worktrees-and-branches.md)
