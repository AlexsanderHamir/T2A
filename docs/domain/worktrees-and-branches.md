# Worktrees, branches, and @-mentions

How registered git worktrees scope agent runs, `/repo/*` autocomplete, and `@`-mention validation in task prompts.

| | |
| --- | --- |
| **Applies to** | `pkgs/gitwork/`, `pkgs/repo/`, git store/handlers, `web/src/worktrees/`, task `worktree_id` |
| **Audience** | Contributors touching git binding, worker `WorkingDir`, or prompt mention validation |
| **Prerequisite** | [ADR-0033](../adr/ADR-0033-git-worktrees-and-branches.md), [ADR-0039](../adr/ADR-0039-fixed-worktree-branch.md), [data-model.md](../data-model.md) (git tables) |
| **Companion articles** | [execute-agent.md](./execute-agent.md), [agent-supervisor.md](./agent-supervisor.md), [cycle-commits.md](./cycle-commits.md) |

## Overview

Hamix scopes workspace access through **registered git worktrees** (`git_worktrees` rows). Each worktree has exactly one immutable `branch_id` (chosen at register/create). Each task carries `worktree_id`; the agent worker resolves the worktree path and bound branch at dequeue, verifies HEAD matches the bound branch (no `git checkout` at pickup), and `@`-mention validation resolves paths against the chosen worktree.

When no git repository is registered:

- The agent worker supervisor stays **idle** (`idle_reason=no_repository_registered`).
- `GET /repo/*` returns **400** without `worktree_id` query param, or **404** for unknown worktree.
- Prompts with `@`-mentions require `worktree_id` on create/patch.

Operators manage repositories, worktrees, and branches on the **`/worktrees`** SPA page (not Settings).

## Operator setup flow (SPA)

Hamix expects operators to follow **repository → worktree (+ branch) → task**:

1. **Register repository** — path to the main git checkout only (no branch field).
2. **Register worktree** or **Create worktree** — pick or add a linked directory and bind a branch in the same step (existing live branch or create new). The branch is fixed for the life of the worktree row.
3. **`/worktrees?register=1`** — deep link that opens the register-repository modal.
4. **Task create gate** — **New task** / **Start fresh** require at least one registered repository.

**Runtime:** tasks on the same worktree run sequentially (per-worktree gate). Tasks on different worktrees may run in parallel when `HAMIX_AGENT_WORKER_CONCURRENCY` > 1. The worker does not switch branches — the worktree must already be checked out on its bound branch.

> **Important** — Workspace trees are **read-only over HTTP**. Mutations happen when the execute agent (or the operator outside Hamix) changes files on disk.

## Key concepts

| Term | Definition |
| --- | --- |
| **Git repository** | A registered main checkout (`git_repositories.path`) |
| **Worktree** | A linked working directory (`git_worktrees`) with fixed `branch_id` — main or added worktree |
| **Branch** | A repo-level ref (`git_branches`); bound to at most one worktree |
| **`WorkingDir`** | `runner.Request.WorkingDir` — task worktree path at dequeue |
| **`@`-mention** | Token in `initial_prompt`: `@path` or `@path(start-end)` |

## Worker and supervisor

- Idle reasons: `no_repository_registered`, `all_worktrees_invalid`, `paused_by_operator` (not a global settings path).
- Pre-run: per-worktree gate (`WorktreeGate`); optional HEAD verify — no checkout at pickup.
- Pool: N queue consumers share one `MemoryQueue` (`HAMIX_AGENT_WORKER_CONCURRENCY`, default 4). Busy worktrees defer pickup via `TryLock` (~5s).
- Delete guard: **409** `has_running_task` when a **running** task targets the worktree or branch.

## HTTP `/repo/*`

All routes require `?worktree_id=`. `RepoProvider.OpenWorktreeRoot` opens the worktree path from the DB.

## Task completion hook

When the harness marks a task `done`, it appends an `on_task_done` audit event with `worktree_id` and `commits[]` (cycle commit index). Foundation for future PR automation — no UI in v0.1.

## See also

- [docs/plans/issue-39-git-workflow-roadmap.md](../plans/issue-39-git-workflow-roadmap.md)
- [api.md](../api.md) — git and `/repo/*` routes
- [configuration.md](../configuration.md) — `HAMIX_PATH_MAP`, `HAMIX_AGENT_WORKER_CONCURRENCY`, Docker mounts
