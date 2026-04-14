# `pkgs/tasks/calltrace`

Per-request **call stack** for structured logs (`call_path`, `helper.io`): `Push`, `Path`, `WithRequestRoot`, `RunObserved`, and `HelperIOIn` / `HelperIOOut`.

**Consumers:** `pkgs/tasks/handler` (HTTP handlers and JSON helpers), `pkgs/tasks/middleware` (access log `call_path` via `Path` passed into `Stack`), `internal/taskapi` (wires `middleware.Stack(..., calltrace.Path)`).

**Dependencies:** stdlib and `log/slog` only in production code; tests use `pkgs/tasks/logctx` for `log_seq` assertions.

| File | Role |
|------|------|
| `const.go` | Shared `LogCmd` (`taskapi`) for slog `cmd` field. |
| `stack.go` | `Push`, `Path`, `WithRequestRoot`. |
| `observe.go` | `RunObserved`, `HelperIOIn`, `HelperIOOut`, internal helper.io emitters. |

See `handler/doc.go` and [docs/OBSERVABILITY.md](../../docs/OBSERVABILITY.md) for usage conventions.
