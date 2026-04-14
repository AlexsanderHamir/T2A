# Scaling `pkgs/tasks/handler`

The **`handler`** package is intentionally **flat**: one directory, one `package handler`, because Go ties package identity to the folder. That keeps `NewHandler`, route registration, and shared test helpers simple, but it also means **many `*_test.go` files live next to production `*.go` files**, which gets noisy as the API surface grows.

This document records **how we keep the tree workable** and **what to do next** when the package feels crowded.

## What we already split out

| Concern | Package | Why it left `handler` |
|---------|---------|------------------------|
| HTTP middleware chain | [`pkgs/tasks/middleware`](../pkgs/tasks/middleware/) | No import cycle with `handler`; `Stack` only needs `apijson` + `logctx`. |
| Black-box middleware tests | [`internal/middlewaretest`](../internal/middlewaretest/) | Keeps `middleware/` smaller; `go test ./...` still runs them. |
| Call stack / `call_path` / helper.io | [`pkgs/tasks/calltrace`](../pkgs/tasks/calltrace/) | Shared by `handler`, `middleware` (access log), and `internal/taskapi` without cycles. |
| JSON truncation at boundaries | [`pkgs/tasks/apijson`](../pkgs/tasks/apijson/) | Shared helpers; `handler` stays focused on routing and mapping. |

See [`pkgs/tasks/handler/README.md`](../pkgs/tasks/handler/README.md) for the file→route map.

## Conventions (defaults for new work)

1. **Prefer small, vertical slices** when adding behavior: `domain` → `store` → `handler` route + tests, per [`EXTENSIBILITY.md`](./EXTENSIBILITY.md).
2. **New tests**
   - **Whitebox** (need unexported symbols, `decodeJSON`, path helpers, fixtures next to `testdata/`): keep **`package handler`** in `pkgs/tasks/handler/`.
   - **Black-box HTTP** (only `NewHandler`, `With*`, `httptest`, `http.Client`): prefer **`internal/handlertest`** (same pattern as `internal/middlewaretest`) once that package exists; until then, colocated `handler` tests are fine.
3. **Do not** create `handler/subpkg` with `package handler` — Go does not allow it. A subdirectory must be a **new import path** (e.g. `pkgs/tasks/taskjson`) with explicit exports and wiring.

## Sensible next extractions (ordered)

When a slice of `handler` has **stable boundaries** and **minimal coupling** to the mux:

1. **Task JSON DTOs + encode/decode helpers** → e.g. `pkgs/tasks/taskjson` (types like `taskCreateJSON`, list params, tree encoding). `handler` would import it; tests that only touch JSON move with the package or stay as `taskjson` tests.
2. **Repo HTTP surface** → thin `pkgs/tasks/repohandler` or keep in `repo_handlers.go` until file size forces a split (see backend bar: avoid 800+ line files).
3. **More black-box tests** → `internal/handlertest` + a tiny shared `NewTaskTestServer(t)` helper there (duplicate of `newTaskTestServer` is acceptable to avoid exporting test-only hooks from `handler`).

Large refactors should ship in **small PRs** (one extraction or one test-directory move at a time) with `go test ./...` and contract doc updates when behavior or JSON changes.

## Related docs

- [`OBSERVABILITY.md`](./OBSERVABILITY.md) — logging, `calltrace`, `funclogmeasure`.
- [`DESIGN.md`](./DESIGN.md) — hub links and limitations.
- [`REORGANIZATION-PLAN.md`](./REORGANIZATION-PLAN.md) — historical phased reorg notes.
