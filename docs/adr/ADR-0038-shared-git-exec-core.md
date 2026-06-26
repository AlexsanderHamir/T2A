# ADR-0038: Shared git-exec core (`pkgs/gitcore`)

**Date:** 2026-06-26
**Status:** Accepted
**Deciders:** Hamix maintainers

## Context

Three packages each implemented `git -C <dir> <args...>` via `os/exec` with nearly
identical stdout/stderr capture and local `execErr` types:

- `pkgs/gitexec` — HTTP commit diff/meta helpers
- `pkgs/gitwork` — worktree and branch management
- `pkgs/agents/harness/internal/git` — harness verify/reset/integrity git I/O

Duplication drifted stderr handling (200-char cap in gitwork only) and made
error-classification helpers reach into three different private error types.

## Decision

Introduce `pkgs/gitcore`, a **stdlib-only** package that owns raw git subprocess
execution:

- `Run(ctx, dir, args...)` — `git -C dir args`, trimmed stdout, or `*ExecError`
- `ExecError` with `Unwrap()`, full `Stderr()` for classification, capped `Error()` text
- `ErrGitMissing` when `exec.ErrNotFound`
- `StderrContains(err, substr)` for adapter-level sentinel mapping

**Dependency direction:**

```
gitexec ──► gitcore
gitwork ──► gitcore
harness/internal/git ──► gitcore
gitcore ──► stdlib only
```

Each adapter keeps its domain sentinels and tracing (`gitwork` slog/calltrace,
harness `GitRepo` test injection, `gitexec` HTTP `ErrNotFound`). gitcore does not
import domain, calltrace, or slog.

## Consequences

### Positive

- One exec implementation; stderr classification uses a single error type.
- Adapters shrink to domain mapping and observability only.
- Harness `GitRepo` interface unchanged for test doubles.

### Negative / Trade-offs

- New top-level `pkgs/` entry (justified: shared infra used by three domains).
- `gitexec` and harness error strings may truncate stderr at 200 chars in
  `Error()` (classification still uses full stderr via `Stderr()`).

## Alternatives Considered

| Alternative | Reason Rejected |
| --- | --- |
| Consolidate into `gitexec` only | Harness and gitwork would import HTTP-oriented package; wrong dependency direction |
| Single `GitService` interface everywhere | Over-engineering; harness needs injectable `GitRepo`, gitwork needs richer sentinels |
| Leave duplication | Already drifted; three copies to fix on every git-exec change |
