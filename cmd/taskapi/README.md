# `cmd/taskapi`

The **`taskapi`** HTTP server binary (`package main`). Contracts and env tables: [docs/RUNTIME-ENV.md](../../docs/RUNTIME-ENV.md), [docs/API-HTTP.md](../../docs/API-HTTP.md). Long-form command doc: [doc.go](./doc.go) (`go doc ./cmd/taskapi`).

## Files

| File | Role |
|------|------|
| `main.go` | `main()` → `run()`; process-wide HTTP server constants (read header/read/idle timeouts, shutdown, max headers). |
| `run.go` | Flag parse, `.env` preload, JSON log bootstrap (**`pkgs/tasks/logctx`** wraps the JSON `slog` handler), DB open/migrate, store + SSE hub + optional repo, agent queue + reconcile goroutine, **`internal/taskapi.NewHTTPHandler`**, devsim ticker, mux (`/` + `GET /metrics`), `ListenAndServe`, graceful shutdown. |
| `run_helpers.go` | `emitTaskAPIFileLoggingConfig` (structured line after JSON handler is live). |
| `logfile.go` | Create log directory and open per-run `taskapi-*.jsonl` (`-logdir` / `T2A_LOG_DIR`). |
| `logging_startup_test.go` | Logging config line shape. |
| `logfile_test.go` | Log path / directory behavior. |

## Dependencies (wiring only)

| Package | Role |
|---------|------|
| [`internal/envload`](../../internal/envload) | Resolve and load `.env`; require `DATABASE_URL`. |
| [`internal/taskapiconfig`](../../internal/taskapiconfig) | Listen host, log level, minimized logging, agent queue cap, reconcile interval, dev SSE ticker interval. |
| [`internal/taskapi`](../../internal/taskapi) | `NewHTTPHandler` → `middleware.Stack(handler.NewHandler(...), calltrace.Path)` (`pkgs/tasks/middleware/stack.go` defines the `With*` order). |
| [`pkgs/tasks/postgres`](../../pkgs/tasks/postgres) | GORM open + `AutoMigrate`. |
| [`pkgs/tasks/store`](../../pkgs/tasks/store) | Persistence; `SetReadyTaskNotifier` for the agent queue. |
| [`pkgs/tasks/handler`](../../pkgs/tasks/handler) | REST + SSE inner mux; see [`handler/README.md`](../../pkgs/tasks/handler/README.md). |
| [`pkgs/tasks/logctx`](../../pkgs/tasks/logctx) | Request id + `log_seq` context and `slog` handler wrappers for JSONL correlation (used in `run.go` and handler). |
| [`pkgs/agents`](../../pkgs/agents) | `MemoryQueue` + reconcile loop. |
| [`pkgs/repo`](../../pkgs/repo) | Optional workspace when `app_settings.repo_root` is set (SPA Settings page; see `docs/SETTINGS.md`). |
| [`pkgs/tasks/devsim`](../../pkgs/tasks/devsim) | Synthetic SSE / audit ticker when `T2A_SSE_TEST=1`. |

Keep **business rules** out of this directory—only startup, wiring, and process lifecycle.
