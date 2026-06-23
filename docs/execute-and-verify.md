# Execute and verify agents

How Hamix runs a task with two agents. Execute implements work; verify judges your checklist. This article explains what your criteria mean in practice.

| | |
| --- | --- |
| **Applies to** | Creating tasks, writing done criteria (checklist items), reviewing failed cycles |
| **Audience** | Operators and anyone defining work for the agent worker |
| **Related articles** | [domain/done-criteria.md](./domain/done-criteria.md), [domain/execute-agent.md](./domain/execute-agent.md), [domain/verify-agent.md](./domain/verify-agent.md) |

## In this article

- [Overview](#overview)
- [One task at a time](#one-task-at-a-time)
- [Execute and verify agents](#execute-and-verify-agents)
- [Report files (behind the scenes)](#report-files-behind-the-scenes)
- [Creating tasks and criteria](#creating-tasks-and-criteria)
- [What happens when a task runs](#what-happens-when-a-task-runs)
- [Dedicated worktree (recommended)](#dedicated-worktree-recommended)
- [Do not edit the workspace during verify](#do-not-edit-the-workspace-during-verify)
- [Writing good criteria](#writing-good-criteria)
- [Retries and locked passes](#retries-and-locked-passes)
- [What you see in the UI](#what-you-see-in-the-ui)
- [See also](#see-also)

## Overview

Every task with done criteria goes through a **two step review**:

1. **Execute agent.** Implements the task and states what it believes it finished.
2. **Verify agent.** Independently checks whether each criterion is actually satisfied.

A task reaches **done** only when verify accepts every active criterion, not when execute merely claims success.

You define the contract when you create the task: the task description and checklist items. The system handles the rest.

> **Important:** New tasks require **at least one** checklist item (done criterion). Each item needs clear, checkable text.

## One task at a time

As of today, you can create as many tasks as you need. There is no cap on how many you can add to the board.

The agent worker still runs **one task at a time**. If you create 100 tasks and they are all ready for the agent, only one is picked up and executed at once. The rest wait in line until the current run finishes (success or failure), then the next eligible task starts.

**Start over** and **Resume from failure** follow the same rule. They only appear when a task has already **failed**, so that task is not running anymore. The action does not start work immediately: it queues the task as ready with your retry choice saved. If another task is executing when you click retry, your task waits in line like any other ready task. Hamix does not block the button because another task is in flight; the single worker prevents two runs at once.

Tasks that are blocked (for example, waiting on dependencies or a deferred pickup time) stay out of the queue until they become ready.

## Execute and verify agents

| Agent | Role | Trusted for final acceptance? |
| --- | --- | --- |
| **Execute** | Reads your task prompt and criteria, changes the repo, commits when required, and reports what it claims to have done. | **No.** Self claim only. |
| **Verify** | Inspects the repo (and optional shell checks you attach), then judges each criterion pass or fail. | **Yes.** Sole authority for marking criteria done on success. |

Execute and verify can use the **same or different** AI runners/models (see Settings). Separation is intentional: one agent builds; another judges.

## Report files (behind the scenes)

While a cycle runs, agents write short JSON reports to a **worker scratch folder** outside your git repo (`HAMIX_WORKER_REPORT_DIR`). You do not create, edit, or open these files.

| File | Written by | Purpose |
| --- | --- | --- |
| `criteria-report.json` | Execute agent | Per criterion **self claim**: `claimed_done` + `evidence`. Optional git `commits[]`. |
| `verify-report.json` | Verify agent | Per criterion **verdict**: `verified` + `reasoning`. |

These files are **temporary**. The worker parses them once, stores durable results in the database, and deletes the scratch folder when the cycle ends. For support and history, use the task UI (checklist, cycle events, verdicts), not the JSON paths.

> **Note:** If execute sets `claimed_done: false` for a criterion, verify is **skipped** for that item and the cycle fails that gate immediately.

## Creating tasks and criteria

When you create a task, you supply:

| Field | What it drives |
| --- | --- |
| **Task description** (`initial_prompt`) | What the execute agent implements. |
| **Done criteria** (checklist items) | Acceptance requirements. Each has a stable `id` and readable `text`. |
| **Verify commands** (optional, per criterion) | Read only shell checks (e.g. `go test ./...`) whose output verify can inspect. Up to five per item. |

**Edit locks:** After the agent picks up the task (`running`), you cannot add or change criterion definitions. Plan acceptance requirements **before** pickup.

**Not the same as release gates:** `task.gate` is a separate operator release mechanism. Done criteria control whether work is accepted as complete.

## What happens when a task runs

```text
1. Execute agent runs in your repo
   → implements the task
   → writes criteria-report.json (self claims per criterion)

2. Gate
   → any claimed_done: false → fail (no verify for that item)

3. Verify agent runs
   → optional shell verify commands run first
   → writes verify-report.json (pass/fail per criterion)

4. Decision
   → all pass → task marked done; checklist completions recorded
   → any fail → retry (up to configured limit) or cycle fails
```

Criteria that already passed on an earlier attempt in the same cycle can be **locked**. Execute is told not to redo them, and verify short circuits them on retry.

## Dedicated worktree (recommended)

Hamix runs execute and verify in one shared directory: **`app_settings.repo_root`** (Settings → Workspace repository). That path is also where `@`-mentions and verify shell commands resolve.

**Recommended:** create a **separate git worktree** for Hamix and set `repo_root` to that directory. Keep your usual checkout for day-to-day edits; let the agent work in the worktree. When a cycle finishes, merge or cherry-pick from the worktree branch as you would for any other feature branch.

From your main repository (replace branch and path as needed):

```bash
# New branch + worktree (example: sibling directory myproject-hamix)
git worktree add ../myproject-hamix -b hamix-agent main

# Or attach a worktree to an existing branch
git worktree add ../myproject-hamix my-feature-branch
```

Then in Hamix Settings, set **Workspace repository** to the absolute path of that worktree (e.g. `/home/you/src/myproject-hamix` or `C:\src\myproject-hamix`).

| Approach | Benefit |
| --- | --- |
| Dedicated worktree as `repo_root` | You can edit your main checkout while Hamix runs; agent commits stay on the worktree branch |
| Main checkout as `repo_root` | Simplest setup, but your saves/commits during verify can fail the cycle (see below) |

Hamix treats a worktree like any other git root — execute commit ingest and verify integrity checks work the same. Per-cycle automatic worktrees are not built in yet; one operator-chosen worktree is the V1 pattern.

Remove a worktree when done: `git worktree remove ../myproject-hamix` (from the main repo).

## Do not edit the workspace during verify

Execute and verify both run in the same directory: **`app_settings.repo_root`** (Settings → Workspace repository). There is no per-task sandbox in V1.

While the **verify** phase is running, the worker snapshots git state before and after judgment. If you save files, commit, checkout, or otherwise change the working tree or `HEAD` during that window, the cycle terminates as **`verify_tampered`** — a hard failure with **no retry**, not a normal verify miss.

| When you edit | Typical outcome |
| --- | --- |
| During **verify** (commits, new edits, checkout) | `verify_tampered` — cycle fails terminally |
| Before verify starts (execute finished, verify not yet running) | Verify may still run, but judges the combined repo state (your edits + the agent's work) |
| During **execute** | No integrity check; verify later sees whatever the tree contains |

**Practical rule:** treat the workspace as read-only from the moment verify starts until the cycle succeeds or fails. If you use a dedicated Hamix worktree (above), edit your **other** checkout freely — only avoid changes inside `repo_root` during verify.

Mechanism and rationale: [domain/verify-agent.md](./domain/verify-agent.md#integrity-enforcement), [ADR-0003](./adr/ADR-0003-verify-component-upgrade.md).

## Writing good criteria

Write criteria that an agent and a separate verifier can evaluate without guesswork.

**Do**

- One clear outcome per item.
- Observable behavior: endpoints, tests, files, commands, status codes.
- Short, specific text tied to the task goal.

**Examples**

- `GET /health returns 200 with {"status":"ok"}.`
- `go test ./pkgs/tasks/handler/... passes.`
- `New handler returns 404 when the task id is missing.`

**Avoid**

- Vague goals: “code is clean”, “feature works”, “looks good”.
- Criteria that require subjective judgment only.
- Destructive verify commands (mutate the tree, install globally, etc.). Use read only checks.

> **Warning:** A verify command exiting 0 does **not** automatically mark a criterion done. The verify agent still makes the final call.

## Retries and locked passes

When verify fails, the cycle retries execute and verify up to `verify_max_retries` (Settings).

On each retry:

- The execute agent gets feedback on what failed.
- Criteria that **already passed** are skipped. Execute does not redo them, and verify does not judge them again.

**When does the checklist update?** Only when the full run succeeds and the task reaches **done**. Until then, a passing criterion is progress inside that run, not a permanent checkmark on the task.

If the run ends in failure (including after all retries), those passes do not stick. On the task detail page, checklist items stay unsatisfied and the task does not move to **done**, even if some criteria passed on every attempt.

## What you see in the UI

| You want to know… | Where to look |
| --- | --- |
| What must be done | Task detail → Done criteria (checklist) |
| Progress during a run | Checklist satisfied counts; cycle / phase events |
| Why verify failed | Cycle events, verification details, verdicts API |
| Whether the task is truly done | Task status `done` after a **succeeded** cycle |

You do not need access to `criteria-report.json` or `verify-report.json` for normal operation.

## See also

| Doc | Why read it |
| --- | --- |
| [data-model.md](./data-model.md) (Checklist) | Schema, edit locks, verdict tables |
| [domain/done-criteria.md](./domain/done-criteria.md) | Full verification loop and wire contracts |
| [domain/execute-agent.md](./domain/execute-agent.md) | Execute prompt and report format |
| [domain/verify-agent.md](./domain/verify-agent.md) | Verify prompt, shell checks, integrity rules |
| [configuration.md](./configuration.md) | Verify retries, runners, `HAMIX_WORKER_REPORT_DIR` |
| [api.md](./api.md) | Create task body (`checklist_items`), checklist routes |
