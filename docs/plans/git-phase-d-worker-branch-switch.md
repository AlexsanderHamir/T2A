# Plan D — Worker branch switch

**Parent:** worktree registration phases index

## Goal

Document and test sequential same-worktree / different-branch task execution.

## Changes

- `TestAgentWorkerE2E_sameWorktreeDifferentBranchesSequential` in agentreconcile
- `docs/domain/worktrees-and-branches.md` operator + runtime copy

## Exit criteria

- Test proves two associations on one worktree run sequentially.
- Docs match worker checkout behavior.
