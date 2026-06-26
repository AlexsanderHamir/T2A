# Plan B — Worktree + branch API

**Parent:** worktree registration phases index

## Goal

Backend supports one-shot worktree + branch binding and live worktree discovery.

## Routes

- `GET /git/repositories/{repoId}/worktrees/live` — linked worktrees from git with `registered` flag
- `POST /git/repositories/{repoId}/worktrees/register` — body includes optional `branch` bind object
- `POST /git/repositories/{repoId}/worktrees` — auto-associates checkout branch after create
- `POST /git/repositories/{repoId}/reconcile` — global reconcile
- Fix `POST /git/worktrees/{worktreeId}/branches` — resolve existing branch by name without always creating

## Store

- `ResolveOrCreateBranchForRepo`, `bindWorktreeBranch` in `store_git_branch_bind.go`
- `RegisterExistingGitWorktree` accepts `BindBranchInput`

## Exit criteria

- API docs updated; store/handler tests green; curl flows work.
