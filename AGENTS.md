# Agent orientation (AI + contributors)

Use this file as the first pass before editing code. Long-form contracts live in `docs/`; this file is a map and checklist.

## Read order

| Order | Doc | Why |
|------|-----|-----|
| 1 | [README.md](README.md) | Install, run `taskapi` / `dbcheck`, `web/` npm commands, dev scripts. |
| 2 | [CONTRIBUTING.md](CONTRIBUTING.md) | PR checklist, `.env.example`, API/doc sync pointers. |
| — | [docs/PRODUCT.md](docs/PRODUCT.md) | Control-plane positioning and what T2A provides before large bets; horizons, scope rules. |
| — | [docs/REORGANIZATION-PLAN.md](docs/REORGANIZATION-PLAN.md) | **Planned** docs + backend layout (phased milestones); read before large structural work. |
| 3 | [docs/DESIGN.md](docs/DESIGN.md) | **Hub:** architecture, limitations, links to contract docs. |
| 4 | [docs/API-HTTP.md](docs/API-HTTP.md) | REST + `/repo`: routes, bodies, errors, metrics. |
| 5 | [docs/API-SSE.md](docs/API-SSE.md) | `GET /events` and dev SSE env. |
| 6 | [docs/RUNTIME-ENV.md](docs/RUNTIME-ENV.md) | Env vars, startup, shutdown, timeouts. |
| 7 | [docs/WEB.md](docs/WEB.md) | `web/src` layout, React Query + SSE, `parseTaskApi`, Vitest. |
| 8 | [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common dev failures (Vite proxy, SSE, `REPO_ROOT`). |
| 9 | [docs/OBSERVABILITY.md](docs/OBSERVABILITY.md) | Logs, correlation, per-function `slog` target (`funclogmeasure`), and test coverage scripts. |

Cursor: `99-repo-primer.mdc` (always-on), `01`–`08`, `docs/API-HTTP.md` / `docs/API-SSE.md` (HTTP/SSE contracts), `13-tasks-stack-extensibility` (tasks API layering), `14-repo-workspace-extensibility` (`REPO_ROOT` / `/repo` / `pkgs/repo`), `15-database-schema` (GORM models / migrate path), `12-documentation-style` (README/docs prose), `09-local-verification` + `09-security-baseline`, `10-web-ui` for `web/`. **`00-full-rules-pass.mdc`** defines scope (default **full repo** unless paths or user intent narrow it; **docs-and-rules-only** skips Go/npm checks), phases, and the completion report—**@-mention that file in Cursor** for large or cross-cutting agent work so the structured pass runs. `06-testing.mdc` defines `go test` expectations; `10-web-ui.mdc` defines `npm test` for `web/`. Test failure triage: `docs/TROUBLESHOOTING.md` (**Local checks and agent test failures**). GitHub Actions (`.github/workflows/ci.yml`) runs a **backend** job (`gofmt`, `go vet`, `go test`, `funclogmeasure -enforce`) and a separate **web** job (`npm ci`, `npm test`, `npm run lint`, `npm run build`); `./scripts/check.sh` / `.\scripts\check.ps1` from the repo root combine both in one local command.

## Repository map

| Area | Path | Notes |
|------|------|--------|
| HTTP API + SSE | `pkgs/tasks/handler/` | REST `/tasks`, `GET /events`, `/repo/*` when `REPO_ROOT` set; `GET /health`, `/health/live`, `/health/ready`; `GET /metrics` (Prometheus). File map: `pkgs/tasks/handler/README.md`. |
| Request log correlation | `pkgs/tasks/logctx/` | `request_id` on context, per-request `log_seq`, `slog.Handler` wrappers; imported by `handler` and `cmd/taskapi` (stdlib-only, no cycle with `handler`). |
| JSON API response helpers | `pkgs/tasks/apijson/` | Shared security headers + `WriteJSONError`; depends on `logctx` only. `handler` delegates `writeJSONError` here (passes `CallPath` for debug). |
| Persistence | `pkgs/tasks/store/`, `pkgs/tasks/postgres/` | Store maps DB errors to `domain.ErrNotFound` / `ErrInvalidInput`. File map: `pkgs/tasks/store/README.md`. |
| Domain types | `pkgs/tasks/domain/` | Status, priority, task model, audit events. |
| Workspace search | `pkgs/repo/` | Optional; used for `@file` mentions when repo configured. |
| Agent hooks | `pkgs/agents/` | In-process ready-task queue always wired from `taskapi` (`store.SetReadyTaskNotifier`); defaults **256** cap and **5m** reconcile interval (env overrides); see `docs/AGENT-QUEUE.md` and `docs/RUNTIME-ENV.md`. |
| Agent reconcile tests | `pkgs/tasks/agentreconcile/` | Integration tests (SQLite store + agents); not imported by production code. |
| Env loading | `internal/envload/` | Resolves `.env` from repo root. |
| taskapi startup env | `internal/taskapiconfig/` | Listen host, log level / minimized logging, agent queue + reconcile interval, dev SSE ticker interval (see `cmd/taskapi/run.go`). |
| taskapi HTTP stack | `pkgs/tasks/handler/stack.go` + `internal/taskapi/` | `handler.MiddlewareStack` composes `With*` layers; `internal/taskapi.NewHTTPHandler` wires store/hub/repo into `handler.NewHandler` then applies the stack (see `cmd/taskapi/run.go`). |
| SQLite test DB | `internal/tasktestdb/` | In-memory GORM + migrate for default store/handler/agent tests (`tasktestdb.OpenSQLite`). |
| Dev UI simulation | `pkgs/tasks/devsim/` | Optional `T2A_SSE_TEST` ticker: synthetic audit, row mirror, user-response sim, lifecycle tasks, burst count + SSE (`cmd/taskapi`); see `docs/API-SSE.md`. |
| Binaries | `cmd/taskapi/`, `cmd/dbcheck/` | Entry points only. `taskapi` file map: [`cmd/taskapi/README.md`](cmd/taskapi/README.md). |
| Web SPA | `web/` | Vite + React; `fetch` only under `web/src/api/`; import `@/types`, `@/api`. Global styles: `web/src/app/App.css` `@import`s partials under `web/src/app/styles/`. |

API contracts (paths, query params, JSON shapes) are authoritative in `docs/API-HTTP.md`, `docs/API-SSE.md`, and `docs/WEB.md` (SPA), not only in prose comments. The `docs/DESIGN.md` hub links limitations and architecture.

## Commands to run before you finish

| Change | Command |
|--------|---------|
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
- Atomic commits: `.cursor/rules/08-atomic-commits.mdc` — one logical concern per commit, conventional message style; push after committing unless the user opts out or push is not possible.
- Docs: When you change flags, routes, or env vars, update the focused doc (`docs/RUNTIME-ENV.md`, `docs/API-HTTP.md`, `docs/API-SSE.md`, or `docs/DESIGN.md` hub for limitations) and `docs/WEB.md` / root `README.md` if user-facing commands change; see `docs/README.md` “Where to put updates”.

## Quick pitfalls

- Do not add `fetch` to `web/src` components for app APIs — use `web/src/api/`.
- Do not rely on `taskapi` serving `web/dist`; production is static files + reverse proxy or same-origin gateway.
- `GET /events` is SSE; `/health` is plain JSON — different clients.
- Default per-IP HTTP rate limit is 120/min (`T2A_RATE_LIMIT_PER_MIN`); set **`0`** to disable for heavy local testing.
