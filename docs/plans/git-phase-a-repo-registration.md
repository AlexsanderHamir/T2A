# Plan A — Repo registration (path only)

**Parent:** [worktree registration phases index](../../.cursor/plans/worktree_registration_phases_eb5059e2.plan.md)

## Goal

Repo registration collects only the main git checkout path. No operator branch input.

## Changes

- `RegisterRepositoryModal`: remove default branch field; submit `{ path }` only.
- Modal copy: repo anchors the checkout; worktrees and branches are registered separately.
- `WorktreesPage`: stop passing `defaultBranch` to create modal.
- Backend: when `default_branch` omitted, auto-detect from current branch at repo root (internal metadata).

## Exit criteria

- Register repository modal shows path picker only.
- `.\scripts\check.ps1 -WebOnly` green.
