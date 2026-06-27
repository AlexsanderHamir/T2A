# pkgs/gitwork

Git worktree and branch operations for Hamix worktree management ([Issue #39](https://github.com/AlexsanderHamir/Hamix/issues/39)).

## Scope

- `OpenRepository` — validate a path is a git root
- `OpenRegisteredCheckout` — open a registered repo from cache, candidate path, or bounded sibling discovery ([git-checkout-resolution.md](../../docs/domain/git-checkout-resolution.md))
- Worktrees — list, add, remove (`git worktree`)
- Branches — list, create, delete, checkout

## Out of scope

- HTTP handlers (Plan 3)
- Task/worker binding (Plan 4)
- Merge, rebase, fetch, pull
- Worktrunk CLI integration

## Usage

```go
svc := gitwork.New()
repo, err := svc.OpenRepository(ctx, "/path/to/main")
wt, err := svc.AddWorktree(ctx, repo, "/path/to/wt", gitwork.AddWorktreeOptions{
    Branch: "feature", CreateBranch: true,
})
```

All paths returned are absolute with forward slashes.

## Tests

```bash
go test ./pkgs/gitwork/... -count=1 -race
```

Requires `git` on PATH (tests skip when absent).
