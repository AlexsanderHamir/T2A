# Agent orientation (AI + contributors)

Use this file as the first pass before editing code. Long-form contracts live in `docs/`; this file is a map and checklist.

## Read order

| Order | Doc | Why |
|------|-----|-----|
| 1 | [README.md](README.md) | Install, run `taskapi` / `dbcheck`, `web/` npm commands, dev scripts. |
| 2 | [CONTRIBUTING.md](CONTRIBUTING.md) | PR checklist, `.env.example`, API/doc sync pointers. |
| — | [docs/PRODUCT.md](docs/PRODUCT.md) | Control-plane positioning and what T2A provides before large bets; horizons, scope rules. |
| — | [docs/REORGANIZATION-PLAN.md](docs/REORGANIZATION-PLAN.md) | **Principles** for the docs + backend layout (dependency rules, non-goals, what-not-to-do); read before large structural work. |
| — | [docs/proposals/](docs/proposals/) | **Forward-looking** design docs for features that have not yet shipped (one file per proposal). |
| 3 | [docs/DESIGN.md](docs/DESIGN.md) | **Hub:** architecture, limitations, links to contract docs. |
| 4 | [docs/API-HTTP.md](docs/API-HTTP.md) | REST + `/repo`: routes, bodies, errors, metrics. |
| 5 | [docs/API-SSE.md](docs/API-SSE.md) | `GET /events` and dev SSE env. |
| — | [docs/EXECUTION-CYCLES.md](docs/EXECUTION-CYCLES.md) | `task_cycles` / `task_cycle_phases` substrate, dual-write invariant, state machine, where reads go. |
| — | [docs/AGENT-WORKER.md](docs/AGENT-WORKER.md) | V1 in-process Cursor CLI worker contract: lifecycle, runner abstraction, security model, audit shape, orphan sweep, deferrals. Configured live via the SPA Settings page; see `docs/SETTINGS.md`. |
| — | [docs/SETTINGS.md](docs/SETTINGS.md) | UI-driven config: singleton `app_settings` row, SPA Settings page, `GET/PATCH /settings`, env-var migration table. |
| 6 | [docs/RUNTIME-ENV.md](docs/RUNTIME-ENV.md) | Env vars, startup, shutdown, timeouts. |
| 7 | [docs/WEB.md](docs/WEB.md) | `web/src` layout, React Query + SSE, `parseTaskApi`, Vitest. |
| 8 | [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common dev failures (Vite proxy, SSE, missing workspace repo). |
| 9 | [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) | Logs, correlation, per-function `slog` target (`funclogmeasure`), and test coverage scripts. |
| — | [docs/CODEBASE-COMMENT-SEARCH-MAP.md](docs/CODEBASE-COMMENT-SEARCH-MAP.md) | Repo blocks and `rg` recipes for auditing comments per [`.cursor/rules/codebase_comments.mdc`](.cursor/rules/codebase_comments.mdc) (not HTTP/API contracts). |

Cursor: `99-repo-primer.mdc` (always-on), `01`–`08`, `docs/API-HTTP.md` / `docs/API-SSE.md` (HTTP/SSE contracts), `13-tasks-stack-extensibility` (tasks API layering), `14-repo-workspace-extensibility` (workspace repo / `/repo` / `pkgs/repo`), `15-database-schema` (GORM models / migrate path), `12-documentation-style` (README/docs prose), `09-local-verification` + `09-security-baseline`, `10-web-ui` for `web/`. **`00-full-rules-pass.mdc`** defines scope (default **full repo** unless paths or user intent narrow it; **docs-and-rules-only** skips Go/npm checks), phases, and the completion report—**@-mention that file in Cursor** for large or cross-cutting agent work so the structured pass runs. `06-testing.mdc` defines `go test` expectations; `10-web-ui.mdc` defines `npm test` for `web/`. Test failure triage: `docs/TROUBLESHOOTING.md` (**Local checks and agent test failures**). GitHub Actions (`.github/workflows/ci.yml`) runs a **backend** job (`gofmt`, `go vet`, `go test`, `funclogmeasure -enforce`) and a separate **web** job (`npm ci`, `npm test`, `npm run lint`, `npm run build`); `./scripts/check.sh` / `.\scripts\check.ps1` from the repo root combine both in one local command.

## Repository map

| Area | Path | Notes |
|------|------|--------|
| HTTP API + SSE | `pkgs/tasks/handler/` | REST `/tasks`, `GET /events`, `/repo/*` when `app_settings.repo_root` is set; `GET /health`, `/health/live`, `/health/ready`; `GET /settings`, `PATCH /settings`, `POST /settings/probe-cursor`, `POST /settings/cancel-current-run`; `GET /metrics` (Prometheus). File map: `pkgs/tasks/handler/README.md`. Scaling and split conventions: `docs/HANDLER-SCALE.md`. |
| Request call stack / helper.io | `pkgs/tasks/calltrace/` | `Push`, `Path`, `WithRequestRoot`, `RunObserved` for `call_path` in logs; used by `handler`, `middleware` (injected path), `internal/taskapi`. README: `pkgs/tasks/calltrace/README.md`. |
| Request log correlation | `pkgs/tasks/logctx/` | `request_id` on context, per-request `log_seq`, `slog.Handler` wrappers; imported by `handler` and `cmd/taskapi` (stdlib-only, no cycle with `handler`). |
| JSON API response helpers | `pkgs/tasks/apijson/` | Shared security headers + `WriteJSONError`; depends on `logctx` only. `handler` delegates `writeJSONError` here (passes `calltrace.Path` for debug). |
| Persistence | `pkgs/tasks/store/`, `pkgs/tasks/postgres/` | Store is a thin facade (`facade_*.go`) over per-domain `internal/<domain>/` packages (tasks, events, checklist, cycles, drafts, eval, ready, stats, health, devmirror, kernel, notify); cross-domain transactions compose via exported `…InTx` helpers and dual-write to `task_events` (see `docs/EXECUTION-CYCLES.md`). Store maps DB errors to `domain.ErrNotFound` / `ErrInvalidInput`. File map: `pkgs/tasks/store/README.md`. |
| Domain types | `pkgs/tasks/domain/` | Status, priority, task model, audit events; plus `TaskCycle` / `TaskCyclePhase` and the `Phase` / `CycleStatus` / `PhaseStatus` enums + `ValidPhaseTransition` for the diagnose → execute → verify → persist substrate. |
| Execution cycles HTTP | `pkgs/tasks/handler/handler_cycles.go` (+ `handler_cycles_json.go`) | `POST/GET /tasks/{id}/cycles`, `GET/PATCH /tasks/{id}/cycles/{cycleId}`, `POST /tasks/{id}/cycles/{cycleId}/phases`, `PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}`. Publishes `task_cycle_changed` SSE events (see `docs/API-SSE.md`); contract pinned in `docs/API-HTTP.md` and `docs/EXECUTION-CYCLES.md`. |
| Workspace search | `pkgs/repo/` | Optional; used for `@file` mentions when repo configured. |
| Agent hooks | `pkgs/agents/` | In-process ready-task queue always wired from `taskapi` (`store.SetReadyTaskNotifier`); defaults **256** cap and **5m** reconcile interval (env overrides); see `docs/AGENT-QUEUE.md` and `docs/RUNTIME-ENV.md`. |
| Agent runner abstraction | `pkgs/agents/runner/` | `Runner` interface + `Request` / `Result` types + typed sentinel errors (`ErrTimeout`, `ErrNonZeroExit`, `ErrInvalidOutput`); pin point for additional CLI adapters (Claude Code, Codex). Worker uses `Runner.Name()` / `Runner.Version()` in cycle audit. Contract: `docs/AGENT-WORKER.md`. |
| Cursor CLI runner adapter | `pkgs/agents/runner/cursor/` | V1 `runner.Runner` implementation: `cursor --print --output-format json`, env allowlist (`PATH` / `HOME` / `USERPROFILE`, `T2A_*` + `DATABASE_URL` denied), secret redaction (`Authorization`, `T2A_*=`, home-path scrub), and `Probe(cursor --version)` used at startup. Contract: `docs/AGENT-WORKER.md`. |
| Programmable test runner | `pkgs/agents/runner/runnerfake/` | In-memory `runner.Runner` for worker + integration tests; not imported by production code. |
| Agent worker (V1) | `pkgs/agents/worker/` | Single-goroutine consumer of `MemoryQueue` driving one cycle/task via the substrate (`StartCycle` → `StartPhase` → `CompletePhase` → `TerminateCycle`); panic + shutdown recovery on a 5s background ctx; `SweepOrphanRunningCycles` runs once at startup to clean rows left running by a previous process. Configured live from the SPA Settings page (`app_settings.worker_enabled` etc — see `docs/SETTINGS.md`). Contract: `docs/AGENT-WORKER.md`. |
| Agent worker supervisor | `cmd/taskapi/run_agentworker.go` | Reads `app_settings`, builds the runner via `pkgs/agents/runner/registry`, probes the binary, starts/stops `worker.Worker`, hot-reloads on `PATCH /settings`, exposes `CancelCurrentRun` / `Reload` / `ProbeRunner` for the HTTP handler. |
| Runner registry | `pkgs/agents/runner/registry/` | Pluggable runner registration + lookup + probe; `cursor` is the only registered runner today. New runners land as one new file each. |
| App settings store | `pkgs/tasks/store/internal/settings/` | Singleton `app_settings` row (id=1) seeded with `domain.DefaultAppSettings`; `GetSettings` / `UpdateSettings` exposed via the store facade and the `/settings` HTTP routes. Contract: `docs/SETTINGS.md`. |
| Agent reconcile tests | `pkgs/tasks/agentreconcile/` | Integration tests (SQLite store + agents); not imported by production code. |
| Env loading | `internal/envload/` | Resolves `.env` from repo root. |
| taskapi startup env | `internal/taskapiconfig/` | Listen host, log level / minimized logging, agent queue + reconcile interval, dev SSE ticker interval (see `cmd/taskapi/run.go`). |
| taskapi HTTP stack | `pkgs/tasks/middleware/` + `pkgs/tasks/handler/middleware_shim.go` + `internal/taskapi/` | `middleware.Stack(inner, calltrace.Path)` composes `With*` layers; `internal/taskapi.NewHTTPHandler` wires store/hub/repo into `handler.NewHandler` then applies the stack (see `cmd/taskapi/run.go`). File map + env table: `pkgs/tasks/middleware/README.md`. Handler tests and `handler.With*` shims re-export middleware for the same package. |
| Middleware black-box tests | `internal/middlewaretest/` | Exported-API-only tests for `pkgs/tasks/middleware` (keeps the middleware package tree smaller; see `middleware/README.md` Tests). |
| Handler black-box HTTP tests | `internal/handlertest/` | Health, metrics scrape, and similar tests using only exported `handler` + `httptest` (see `internal/handlertest/README.md`). |
| Prometheus rules + alert runbooks | `deploy/prometheus/` + `docs/runbooks/` | Recording/alert YAML for `taskapi` metrics (`t2a-taskapi-rules.yaml`); operator runbook stubs linked from `docs/OBSERVABILITY.md`. |
| HTTP baseline header assertions | `internal/httpsecurityexpect/` | Shared test helper for security headers; imported by `handlertest` and `handler` tests (no import cycle with `handler`). |
| SQLite test DB | `internal/tasktestdb/` | In-memory GORM + migrate for default store/handler/agent tests (`tasktestdb.OpenSQLite`). |
| Dev UI simulation | `pkgs/tasks/devsim/` | Optional `T2A_SSE_TEST` ticker: synthetic audit, row mirror, user-response sim, lifecycle tasks, burst count + SSE (`cmd/taskapi`); see `docs/API-SSE.md`. |
| Binaries | `cmd/taskapi/`, `cmd/dbcheck/` | Entry points only. `taskapi` file map: [`cmd/taskapi/README.md`](cmd/taskapi/README.md). |
| Web SPA | `web/` | Vite + React; `fetch` only under `web/src/api/`; import `@/types`, `@/api`. Task UI under `web/src/tasks/components/` groups families (`task-list/`, `task-create-modal/`, etc.) with per-folder `index.ts` barrels — see **`tasks/components/` layout** in [docs/WEB.md](docs/WEB.md). Global styles: `web/src/app/App.css` `@import`s partials under `web/src/app/styles/`. |

API contracts (paths, query params, JSON shapes) are authoritative in `docs/API-HTTP.md`, `docs/API-SSE.md`, and `docs/WEB.md` (SPA), not only in prose comments. The `docs/DESIGN.md` hub links limitations and architecture.

## Commands to run before you finish

| Change | Command |
|--------|---------|
| **Agent session produced git changes** | Before ending the turn: `git status` → `git add` (scoped) → `git commit` (conventional message, one logical concern) → `git push origin` on the current branch (often `main`). Skip only if the user opted out or push is not possible (no remote, no auth). |
| Full bar (recommended) | From repo root: `.\scripts\check.ps1` (Windows) or `./scripts/check.sh` (Unix). Go-only fast path: set `CHECK_SKIP_WEB=1` (bash) or `$env:CHECK_SKIP_WEB='1'` (PowerShell) to skip `web/` steps. After `go test`, the check scripts run `go run ./cmd/funclogmeasure -enforce` (see `docs/OBSERVABILITY.md`); set `CHECK_SKIP_FUNCLOG=1` to skip that audit locally if needed. |
| Go production code or tests | `go vet ./...`, then `go test ./... -count=1` (from repo root); format touched `*.go` with `gofmt` or `go fmt`. |
| Meaningful `web/` change | `cd web && npm test -- --run && npm run lint && npm run build` |
| Coverage / quality (Go libs) | See `.cursor/rules/06-testing.mdc` (`coverprofile` on `pkgs/...` `internal/...`) |
| Observability measurement | Per-function `slog`: `./scripts/measure-func-slog.sh` or `.\scripts\measure-func-slog.ps1`; test coverage profile: `./scripts/measure-observability.sh` or `.\scripts\measure-observability.ps1` ([docs/OBSERVABILITY.md](docs/OBSERVABILITY.md)) |

Default tests must not require real Postgres, real outbound network, or a running `taskapi` (see `06-testing.mdc` and `10-web-ui.mdc`).

**TDD default for agents:** For bugs and features, add or adjust a **failing** test first, then implement until green (`06-testing.mdc` for Go, `10-web-ui.mdc` for `web/`). See `00-full-rules-pass.mdc` phase **2** when running a full pass.

## Conventions worth remembering

- New tasks API features: follow `docs/EXTENSIBILITY.md` (domain → store → handler → optional `web/`). Rule `.cursor/rules/13-tasks-stack-extensibility.mdc` expands the same slice for agents.
- JSON at the boundary: Web treats responses as `unknown` until `parseTaskApi` validates; keep that pipeline when adding fields.
- Same-origin in prod: `taskapi` does not add CORS; dev uses Vite proxy (`web/vite.config.ts`).
- Atomic commits: `.cursor/rules/08-atomic-commits.mdc` (when present) — one logical concern per commit, conventional message style; **push after commit** unless the user opts out or push is not possible.
- **End of each coding/agent chat:** if tracked files changed, **commit and push** as above before finishing (so `main` and teammates stay in sync); combine with the “Commands to run before you finish” table for checks.
- Docs: When you change flags, routes, or env vars, update the focused doc (`docs/RUNTIME-ENV.md`, `docs/API-HTTP.md`, `docs/API-SSE.md`, or `docs/DESIGN.md` hub for limitations) and `docs/WEB.md` / root `README.md` if user-facing commands change; see `docs/README.md` “Where to put updates”.

## Quick pitfalls

- Do not add `fetch` to `web/src` components for app APIs — use `web/src/api/`.
- Do not rely on `taskapi` serving `web/dist`; production is static files + reverse proxy or same-origin gateway.
- `GET /events` is SSE; `/health` is plain JSON — different clients.
- Default per-IP HTTP rate limit is 120/min (`T2A_RATE_LIMIT_PER_MIN`); set **`0`** to disable for heavy local testing.
