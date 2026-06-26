# Plan C — Worktree UI

**Parent:** worktree registration phases index

## Goal

Operator UI: Register worktree vs Create worktree with inline branch pickers.

## Changes

- `RepositoryCard`: Register worktree + Create worktree buttons
- `RegisterWorktreeModal`: live worktree picker + branch bind
- `CreateWorktreeModal`: checkout branch picker (no default_branch)
- `WorktreeBranchBindFields` shared component
- `WorktreeRow`: Add branch for extra associations

## Exit criteria

- Full operator flow without manual associate step for initial branch.
- `.\scripts\check.ps1 -WebOnly` green.
