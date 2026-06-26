# Worktrees, branches, and @-mentions

How registered git worktrees scope agent runs, `/repo/*` autocomplete, and `@`-mention validation in task prompts.

| | |
| --- | --- |
| **Applies to** | `pkgs/gitwork/`, `pkgs/repo/`, git store/handlers, `web/src/worktrees/`, task `worktree_branch_id` |
| **Audience** | Contributors touching git binding, worker `WorkingDir`, or prompt mention validation |
| **Prerequisite** | [ADR-0033](../adr/ADR-0033-git-worktrees-and-branches.md), [data-model.md](../data-model.md) (git tables) |
| **Companion articles** | [execute-agent.md](./execute-agent.md), [agent-supervisor.md](./agent-supervisor.md), [cycle-commits.md](./cycle-commits.md) |

## Overview

Hamix scopes workspace access through **registered git worktrees** (`git_worktrees` rows). Each task carries `worktree_branch_id` (FK to `worktree_branches`); the agent worker resolves the worktree path and branch at dequeue, runs `gitwork.Checkout` for the bound branch, and `@`-mention validation resolves paths against the chosen worktree.

When no git repository is registered:

- The agent worker supervisor stays **idle** (`idle_reason=no_repository_registered`).
- `GET /repo/*` returns **400** without `worktree_id` query param, or **404** for unknown worktree.
- Prompts with `@`-mentions require `worktree_branch_id` on create/patch.

Operators manage repositories, worktrees, and branches on the **`/worktrees`** SPA page (not Settings).

## Operator setup flow (SPA)

Hamix expects operators to follow **repository → worktree (+ branch) → task**:

1. **Register repository** — path to the main git checkout only (no branch field).
2. **Register worktree** or **Create worktree** — pick or add a linked directory and bind a branch in the same step (existing live branch or create new).
3. **Add branch** (optional) — register additional branch associations on an existing worktree when tasks need the same directory on different branches over time.
4. **`/worktrees?register=1`** — deep link that opens the register-repository modal.
5. **Task create gate** — **New task** / **Start fresh** require at least one registered repository.

**Runtime:** a worktree checks out one branch at a time. Sequential tasks on the same worktree may use different `worktree_branch_id` values; the worker runs `git checkout` before each run (`pkgs/agents/worker/git_run.go`), not the harness.

> **Important** — Workspace trees are **read-only over HTTP**. Mutations happen when the execute agent (or the operator outside Hamix) changes files on disk.

## Key concepts

| Term | Definition |
| --- | --- |
| **Git repository** | A registered main checkout (`git_repositories.path`) for a project |
| **Worktree** | A linked working directory (`git_worktrees`) — main or added worktree |
| **Branch** | A tracked branch record (`git_branches`) tasks bind to |
| **`WorkingDir`** | `runner.Request.WorkingDir` — task worktree path at dequeue |
| **`@`-mention** | Token in `initial_prompt`: `@path` or `@path(start-end)` |

## Worker and supervisor

- Idle reasons: `no_repository_registered`, `all_worktrees_invalid`, `paused_by_operator` (not a global settings path).
- Pre-run: per-worktree mutex; `git checkout` for the task branch before harness.
- Delete guard: **409** `has_running_task` when a **running** task targets the worktree or branch.

## HTTP `/repo/*`

All routes require `?worktree_id=`. `RepoProvider.OpenWorktreeRoot` opens the worktree path from the DB.

## Task completion hook

When the harness marks a task `done`, it appends an `on_task_done` audit event with `worktree_branch_id` and `commits[]` (cycle commit index). Foundation for future PR automation — no UI in v0.1.

## See also

- [docs/plans/issue-39-git-workflow-roadmap.md](../plans/issue-39-git-workflow-roadmap.md)
- [api.md](../api.md) — git and `/repo/*` routes
- [configuration.md](../configuration.md) — `HAMIX_PATH_MAP`, Docker mounts
