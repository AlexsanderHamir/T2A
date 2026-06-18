# Agent orientation (AI + contributors)

Use this file as the first pass before editing code. Contributor reference lives in `docs/`; this file is a map and checklist.

## Read order

| Order | Doc | Why |
|------|-----|-----|
| 1 | [README.md](README.md) | Install, run `taskapi` / `dbcheck`, `web/` npm commands, dev scripts. |
| 2 | [CONTRIBUTING.md](CONTRIBUTING.md) | PR checklist, `.env.example`, API/doc sync pointers. |
| 3 | [docs/architecture.md](docs/architecture.md) | System overview, store, agent worker, SSE hub, limitations. |
| 4 | [docs/data-model.md](docs/data-model.md) | Tasks, projects, execution cycles/phases, checklist, dependencies, gates. |
| 4b | [docs/domain/](docs/domain/) | Behavioral deep-dives (scheduling, persistence, SSE, queue, supervisor, harness, …). Index: [docs/domain/README.md](docs/domain/README.md). |
| 5 | [docs/api.md](docs/api.md) | REST + SSE endpoint list. Handler code is authoritative for status codes and error strings. |
| 6 | [docs/configuration.md](docs/configuration.md) | Env vars + `app_settings` row. |
| 7 | [docs/web.md](docs/web.md) | `web/src` layout, React Query + SSE, `parseTaskApi`, Vitest. |
| 8 | [docs/contributing.md](docs/contributing.md) | Vertical-slice flow, handler split rules, local troubleshooting. |
| — | [docs/adr/](docs/adr/) | Historical architecture decisions. |

Cursor rules are grouped by purpose under `.cursor/rules/`: shared structure and comments (`CODE_STANDARDS.mdc`, `codebase_comments.mdc`), backend automation (`BACKEND_AUTOMATION/`), UI automation (`UI_AUTOMATION/`), bug hunting (`BUG_HUNTING/`), and feature/product guidance (`FEATURE_IMPLEMENTATION/`). API contracts remain authoritative in `docs/api.md`; web structure and testing expectations live in `docs/web.md` plus `UI_AUTOMATION/testing-recipes.mdc`. Test failure triage: `docs/contributing.md` (**Local checks fail — quick playbook**). GitHub Actions (`.github/workflows/ci.yml`) runs a **backend** job (`gofmt`, `go vet`, `go test`, `funclogmeasure -enforce`) and a separate **web** job (`npm ci`, `npm test`, `npm run lint`, `npm run check:standards`, `npm run build`); `./scripts/check.sh` / `.\scripts\check.ps1` combine both locally.

## Repository map

| Area | Path | Notes |
|------|------|--------|
| HTTP API + SSE | `pkgs/tasks/handler/` | REST `/tasks`, `GET /events`, `/repo/*` when `app_settings.repo_root` is set; `/health*`, `/settings*`, `/metrics`. SSE deep dive: [docs/domain/sse-hub.md](docs/domain/sse-hub.md). File map: `pkgs/tasks/handler/README.md`. Split conventions: [docs/contributing.md](docs/contributing.md). |
| Request call stack / helper.io | `pkgs/tasks/calltrace/` | `Push`, `Path`, `WithRequestRoot`, `RunObserved` for `call_path` in logs. README: `pkgs/tasks/calltrace/README.md`. |
| Request log correlation | `pkgs/tasks/logctx/` | `request_id` on context, per-request `log_seq`, `slog.Handler` wrappers; stdlib-only, no cycle with `handler`. |
| JSON API response helpers | `pkgs/tasks/apijson/` | Shared security headers + `WriteJSONError`; depends on `logctx` only. |
| Persistence | `pkgs/tasks/store/`, `pkgs/tasks/postgres/` | Thin facade over `internal/<domain>/`; dual-write to `task_events`. Deep dive: [docs/domain/persistence.md](docs/domain/persistence.md). File map: `pkgs/tasks/store/README.md`. |
| Domain types | `pkgs/tasks/domain/` | Status, priority, task model, audit events; `TaskCycle` / `TaskCyclePhase` + `Phase` / `CycleStatus` / `PhaseStatus` enums + `ValidPhaseTransition`. |
| Execution cycles HTTP | `pkgs/tasks/handler/handler_cycles.go` (+ `handler_cycles_json.go`) | `POST/GET /tasks/{id}/cycles`, `GET/PATCH /tasks/{id}/cycles/{cycleId}`, `POST /tasks/{id}/cycles/{cycleId}/phases`, `PATCH /tasks/{id}/cycles/{cycleId}/phases/{phaseSeq}`. Publishes `task_cycle_changed`; contract in [docs/api.md](docs/api.md) and [docs/data-model.md](docs/data-model.md). |
| Workspace search | `pkgs/repo/` | Optional; `@`-mentions and `/repo/*` when `app_settings.repo_root` is set. Deep dive: [docs/domain/workspace-repo.md](docs/domain/workspace-repo.md). |
| Agent hooks | `pkgs/agents/` | In-process ready-task queue (`store.SetReadyTaskNotifier`); default **256** cap (`T2A_USER_TASK_AGENT_QUEUE_CAP`); fixed **2m** reconcile tick (`ReconcileTickInterval`, not env). Deep dive: [docs/domain/agent-queue.md](docs/domain/agent-queue.md). |
| Agent runner abstraction | `pkgs/agents/runner/` | `Runner` interface + typed sentinel errors (`ErrTimeout`, `ErrNonZeroExit`, `ErrInvalidOutput`); pin point for additional CLI adapters (Claude Code, Codex). |
| Runner adapter kit | `pkgs/agents/runner/adapterkit/` | Shared CLI adapter mechanics: exec/stream execution, env policy, redaction, diagnostics, probes. |
| Cursor CLI runner adapter | `pkgs/agents/runner/cursor/` | V1 `runner.Runner` implementation: `cursor --print --output-format stream-json`, env allowlist, secret redaction, live progress normalization, `Probe(cursor --version)`. |
| Programmable test runner | `pkgs/agents/runner/runnerfake/` | In-memory `runner.Runner` for tests; not imported by production code. |
| Agent harness | `pkgs/agents/harness/` | Cycle choreography around `runner.Run`: execute/verify phase loop, criteria injection, verification pipeline, git integrity, crash/shutdown recovery. Called by the worker after admission. See [docs/domain/harness.md](docs/domain/harness.md) and [ADR-0005](docs/adr/ADR-0005-extract-agent-harness.md). |
| Cycle commit tracking | `pkgs/agents/harness/git_commits.go`, `pkgs/tasks/store/internal/commits/` | Git ancestry snapshot, execute ingest gates, `task_cycle_commits`, verify/resume prompt blocks, `GET .../verdicts` `git_context` + `commits[]`. Deep dive: [docs/domain/cycle-commits.md](docs/domain/cycle-commits.md), [ADR-0014](docs/adr/ADR-0014-cycle-commit-tracking.md). |
| Agent worker (V1) | `pkgs/agents/worker/` | Single-goroutine consumer of `MemoryQueue` (admission + ack ordering); delegates cycle body to `harness`; `SweepOrphanRunningCycles` runs once at startup. Configured live from the SPA Settings page — see [docs/configuration.md](docs/configuration.md). |
| Agent worker supervisor | `cmd/taskapi/run_agentworker.go` | Boot/reload worker from `app_settings`; probe, hot-swap, cancel. Deep dive: [docs/domain/agent-supervisor.md](docs/domain/agent-supervisor.md). |
| Runner registry | `pkgs/agents/runner/registry/` | Pluggable runner registration + lookup + probe; production `cursor`, scaffold `claude-code`. See [docs/domain/runner-adapters.md](docs/domain/runner-adapters.md). |
| App settings store | `pkgs/tasks/store/internal/settings/` | Singleton `app_settings` row (id=1) seeded with `domain.DefaultAppSettings`; `GetSettings` / `UpdateSettings` via the store facade. |
| Agent reconcile tests | `pkgs/tasks/agentreconcile/` | Integration tests (SQLite store + agents); not imported by production code. |
| Env loading | `internal/envload/` | Resolves `.env` from repo root. |
| taskapi startup env | `internal/taskapiconfig/` | Listen host, log level / minimized logging, agent queue cap, dev SSE ticker interval. |
| taskapi HTTP stack | `pkgs/tasks/middleware/` + `pkgs/tasks/handler/middleware_shim.go` + `internal/taskapi/` | `middleware.Stack(inner, calltrace.Path)` composes `With*` layers; `internal/taskapi.NewHTTPHandler` wires store/hub/repo into `handler.NewHandler` and applies the stack. File map: `pkgs/tasks/middleware/README.md`. |
| Middleware black-box tests | `internal/middlewaretest/` | Exported-API-only tests for `pkgs/tasks/middleware`. |
| Handler black-box HTTP tests | `internal/handlertest/` | Health, metrics, and similar tests using only exported `handler` + `httptest`. |
| HTTP baseline header assertions | `internal/httpsecurityexpect/` | Shared test helper for security headers. |
| SQLite test DB | `internal/tasktestdb/` | In-memory GORM + migrate for default tests (`tasktestdb.OpenSQLite`). |
| Dev UI simulation | `pkgs/tasks/devsim/` | Optional `T2A_SSE_TEST` ticker: synthetic audit, row mirror, user-response sim, lifecycle tasks. |
| Binaries | `cmd/taskapi/`, `cmd/dbcheck/` | Entry points only. `taskapi` file map: [`cmd/taskapi/README.md`](cmd/taskapi/README.md). |
| Web SPA | `web/` | Vite + React; `fetch` only under `web/src/api/`; import `@/types`, `@/api`. Task UI under `web/src/tasks/components/` groups families with per-folder `index.ts` barrels — see [docs/web.md](docs/web.md). Global styles: `web/src/app/App.css` `@import`s partials under `web/src/app/styles/`. |

API contracts (paths, query params, JSON shapes) are authoritative in [docs/api.md](docs/api.md) (and `pkgs/tasks/handler/` godoc for exhaustive behavior). [docs/architecture.md](docs/architecture.md) is the system overview.

## Commands to run before you finish

| Change | Command |
|--------|---------|
| Full bar (recommended) | From repo root: `.\scripts\check.ps1` (Windows) or `./scripts/check.sh` (Unix). Go-only fast path: `CHECK_SKIP_WEB=1` (bash) or `$env:CHECK_SKIP_WEB='1'` (PowerShell). After `go test`, the check scripts run `go run ./cmd/funclogmeasure -enforce`; set `CHECK_SKIP_FUNCLOG=1` to skip locally. |
| Go production code or tests | `go vet ./...`, then `go test ./... -count=1`; format touched `*.go` with `gofmt` or `go fmt`. |
| Meaningful `web/` change | `cd web && npm test -- --run && npm run lint && npm run check:standards && npm run build` |

Default tests must not require real Postgres, real outbound network, or a running `taskapi` (see `.cursor/rules/BACKEND_AUTOMATION/go-testing-recipes.mdc` and `.cursor/rules/UI_AUTOMATION/testing-recipes.mdc`).

**TDD default for agents:** for bugs and features, add or adjust a **failing** test first, then implement until green.

## Conventions worth remembering

- New tasks API features: follow [docs/contributing.md](docs/contributing.md) (domain → store → handler → optional `web/`) and the backend rules under `.cursor/rules/BACKEND_AUTOMATION/`.
- JSON at the boundary: Web treats responses as `unknown` until `parseTaskApi` validates; keep that pipeline when adding fields.
- Same-origin in prod: `taskapi` does not add CORS; dev uses Vite proxy (`web/vite.config.ts`).
- Commits: when the user asks for a commit, keep it to one logical concern with a conventional message and push only when requested.
- Docs: update the focused contributor doc when behavior changes — [docs/README.md](docs/README.md) is the index.

## Quick pitfalls

- Do not add `fetch` to `web/src` components for app APIs — use `web/src/api/`.
- Do not rely on `taskapi` serving `web/dist`; production is static files + reverse proxy or same-origin gateway.
- `GET /events` is SSE; `/health` is plain JSON — different clients.
- Default per-IP HTTP rate limit is 120/min (`T2A_RATE_LIMIT_PER_MIN`); set **`0`** to disable for heavy local testing.
