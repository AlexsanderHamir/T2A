# Agent orientation

First pass before editing code. **Do not read everything** — pick a scoped path below, then open only the linked docs and map rows you need.

Human learning path: [docs/guide.md](docs/guide.md). Doc index: [docs/README.md](docs/README.md). Code paths: [docs/agent-map.md](docs/agent-map.md).

## How to use this file

1. Match your task to **Scoped paths** (read only that row's docs).
2. For a one-off question, use **Where to find X**.
3. Open [docs/agent-map.md](docs/agent-map.md) for the 1–3 rows that match your edit.
4. Run **Commands to run before you finish** when done.

## Scoped paths

| If you are… | Read (in order) | Skip |
| --- | --- | --- |
| Changing Go REST / handlers | [docs/api.md](docs/api.md), [pkgs/tasks/handler/README.md](pkgs/tasks/handler/README.md) | harness docs, [docs/web.md](docs/web.md) |
| Changing Go domain / store | [docs/data-model.md](docs/data-model.md), [pkgs/tasks/store/README.md](pkgs/tasks/store/README.md) | web, harness |
| Changing agent worker / harness | [docs/domain/harness.md](docs/domain/harness.md), [docs/domain/harness-testing.md](docs/domain/harness-testing.md), [docs/configuration.md](docs/configuration.md) | [docs/web.md](docs/web.md) |
| Changing tests / CI / coverage | [docs/domain/testing.md](docs/domain/testing.md), [CONTRIBUTING.md](CONTRIBUTING.md) | web-only work |
| Changing web UI only | [docs/web.md](docs/web.md), `.cursor/rules/frontend_bar.mdc` | architecture, harness |
| Changing web data (API / sync / mutations) | [docs/web.md](docs/web.md) §Task sync / Query policy, `web/src/api/` | handler split guide |
| Adding a full-stack feature | [pkgs/tasks/handler/README.md](pkgs/tasks/handler/README.md), [docs/domain/persistence.md](docs/domain/persistence.md), [docs/api.md](docs/api.md) | — |
| Writing operator / checklist copy | [docs/execute-and-verify.md](docs/execute-and-verify.md) | code map |
| Docs or config only | Target doc from [docs/README.md](docs/README.md) | code map |

## Where to find X

Intent-based lookup. For subsystem inventory, use [docs/agent-map.md](docs/agent-map.md). When landing a new vertical slice, add one row here **or** one row in agent-map — not both unless the feature is high-traffic.

### Navigation

| I need to… | Go to |
| --- | --- |
| Find any subsystem code path | [docs/agent-map.md](docs/agent-map.md) |
| Understand doc structure | [docs/guide.md](docs/guide.md) |
| Pick a doc by topic | [docs/README.md](docs/README.md) |
| PR checklist | [CONTRIBUTING.md](CONTRIBUTING.md#before-you-open-a-pr) |
| Test failure triage | [CONTRIBUTING.md](CONTRIBUTING.md#stuck) |
| Local dev / install | [README.md](README.md) |

### Backend — API, domain, persistence

| I need to… | Go to |
| --- | --- |
| Add or change a REST route | `pkgs/tasks/handler/handler_*.go`, [docs/api.md](docs/api.md) |
| Add DB persistence | `pkgs/tasks/store/`, [docs/domain/persistence.md](docs/domain/persistence.md) |
| Change task JSON shape | `pkgs/tasks/domain/`, `handler_*_json.go`, `web/src/api/parseTaskApi*.ts` |
| Wire SSE after a write | handler `notifyChange`, [docs/domain/sse-hub.md](docs/domain/sse-hub.md) |
| Middleware change | `pkgs/tasks/middleware/`, `internal/middlewaretest/` |
| Change task status or transitions | `pkgs/tasks/domain/`, [docs/data-model.md](docs/data-model.md) |
| Task scheduling / when worker picks up | `pkgs/tasks/scheduling/`, [docs/domain/task-scheduling.md](docs/domain/task-scheduling.md) |
| Checklist API | `handler_checklist.go`, `handler_create_checklist.go`, [docs/domain/done-criteria.md](docs/domain/done-criteria.md) |
| Task dependencies or release gates | `handler_task_dependencies.go`, `handler_depends_on_json.go`, `handler_task_gate.go`, [docs/data-model.md](docs/data-model.md) |
| Bootstrap / list read limits | `handler_bootstrap.go`, `readpolicy/`, [ADR-0026](docs/adr/ADR-0026-backend-data-coherence.md) |
| Execution cycles API | `handler_cycles.go`, `handler_cycles_json.go`, [docs/api.md](docs/api.md) |
| Commits or repo diff API | `handler_commits.go`, `handler_http_repo*.go`, [docs/domain/cycle-commits.md](docs/domain/cycle-commits.md) |
| Task drafts API | `handler_task_drafts.go`, [docs/api.md](docs/api.md) |
| Projects API | `handler_projects.go`, `handler_projects_json.go` |
| Workspace repo / `@`-mentions | `pkgs/repo/`, [docs/domain/worktrees-and-branches.md](docs/domain/worktrees-and-branches.md) |
| Schema migration | `scripts/migrate.*`, `pkgs/tasks/postgres/schema_revision.go`, `go run ./cmd/dbcheck -migrate` — [docs/configuration.md](docs/configuration.md) (Schema migrations), [ADR-0034](docs/adr/ADR-0034-opt-in-schema-migration.md) |
| Write policy / enriched SSE payload | `writepolicy/`, `handler_writepolicy.go`, [ADR-0026](docs/adr/ADR-0026-backend-data-coherence.md) |

### Agents and worker

| I need to… | Go to |
| --- | --- |
| Agent run / verify loop | `pkgs/agents/harness/`, [docs/domain/harness.md](docs/domain/harness.md), [docs/domain/harness-testing.md](docs/domain/harness-testing.md) |
| Worker queue / pickup | `pkgs/agents/worker/`, [docs/domain/agent-queue.md](docs/domain/agent-queue.md) |
| Execute agent prompt / criteria report | [docs/domain/execute-agent.md](docs/domain/execute-agent.md), `pkgs/agents/harness/` execute paths |
| Verify agent / judge pipeline | [docs/domain/verify-agent.md](docs/domain/verify-agent.md), harness verify paths |
| Cursor CLI runner adapter | `pkgs/agents/runner/cursor/`, [docs/domain/runner-adapters.md](docs/domain/runner-adapters.md) |
| Add or register a runner | `pkgs/agents/runner/registry/`, [docs/domain/runner-adapters.md](docs/domain/runner-adapters.md) |
| Operator retry (Start over / Resume) | `handler_tasks_retry.go`, `harness/retry_run.go`, [retry-start-over.md](docs/domain/retry-start-over.md) / [retry-resume.md](docs/domain/retry-resume.md) |
| Cycle commit indexing | `harness/internal/git/commits.go`, [docs/domain/cycle-commits.md](docs/domain/cycle-commits.md) |
| Cursor session `--resume` | `harness/cursor_resume.go`, [docs/domain/cursor-session-resume.md](docs/domain/cursor-session-resume.md) |
| Worker supervisor / hot reload | `internal/taskapi/agentworker/`, [docs/domain/agent-supervisor.md](docs/domain/agent-supervisor.md) |
| Project context in harness | [docs/domain/project-context.md](docs/domain/project-context.md) |
| Audit timeline / task events | `handler_task_events.go`, [docs/domain/task-events.md](docs/domain/task-events.md) |

### Web — data and UI

| I need to… | Go to |
| --- | --- |
| Fix live UI not updating | `web/src/tasks/sync/`, [docs/web.md](docs/web.md) §Task sync |
| Add a fetch call | `web/src/api/` only — never components |
| Add a page or route | `web/src/app/Router.tsx`, `web/src/tasks/pages/` |
| Task templates UI or API | `web/src/api/taskTemplates.ts`, `TaskTemplatesPage.tsx`, `handler_task_templates*.go` |
| Create or edit task modal | `web/src/tasks/create/`, `task-create-modal/` |
| Execution cycles UI | `web/src/tasks/components/task-detail/` (cycles panel) |
| Checklist UI mutations | `web/src/tasks/checklist/`, [docs/web.md](docs/web.md) §Query policy |
| Optimistic task writes / mutation guard | `web/src/tasks/mutations/`, `web/src/tasks/sync/mutationGuard.ts`, [ADR-0025](docs/adr/ADR-0025-frontend-data-coherence.md) |
| Task list / home table | `web/src/tasks/components/task-list/` |
| When task fields are editable | `web/src/tasks/task-display/` (`canEditTask`, etc.) |
| Commit diff page | `TaskCommitDiffPage.tsx`, [docs/web.md](docs/web.md) §Task detail |
| Cold start / bootstrap query | `web/src/app/hooks/useBootstrap.ts`, [docs/web.md](docs/web.md) §Cold start |
| Task drafts UI | `TaskDraftsPage.tsx`, `web/src/api/parseTaskApiDrafts.ts` |
| Projects UI | `web/src/projects/` |
| Settings page | `web/src/settings/` |
| SSE transport hook (not sync policy) | `web/src/tasks/hooks/useTaskEventStream.ts`, [docs/domain/sse-hub.md](docs/domain/sse-hub.md) |
| Vitest / MSW test setup | `web/src/test/`, `.cursor/rules/UI_AUTOMATION/testing-recipes.mdc` |
| Go test tiers, CI groups, coverage gate | [docs/domain/testing.md](docs/domain/testing.md), `scripts/test-groups.sh`, `scripts/coverage-baselines.json` |
| Write operator / checklist copy | [docs/execute-and-verify.md](docs/execute-and-verify.md), [docs/domain/done-criteria.md](docs/domain/done-criteria.md) |
| UI tokens or spacing | `web/src/app/styles/tokens/`, `frontend_bar.mdc` |
| Hidden launch features | [docs/omitted-features.md](docs/omitted-features.md) |

### Observability and local ops

| I need to… | Go to |
| --- | --- |
| Structured logs / `request_id` | `pkgs/tasks/logctx/`, `pkgs/tasks/calltrace/` |
| Fix `funclogmeasure -enforce` failure | [docs/domain/observability-trace-lines.md](docs/domain/observability-trace-lines.md) |
| Match a failing request in logs | [CONTRIBUTING.md](CONTRIBUTING.md#stuck) |
| Dev SSE ticker for local UI | `pkgs/tasks/devsim/`, `HAMIX_SSE_TEST` in [docs/configuration.md](docs/configuration.md) |
| Why a design was chosen | [docs/adr/](docs/adr/) (not for day-to-day routes) |

### Engineering meta

| I need to… | Go to |
| --- | --- |
| Where a new file goes | `.cursor/rules/CODE_STANDARDS.mdc` Part 12 |
| Handler file too large | [pkgs/tasks/handler/README.md](pkgs/tasks/handler/README.md) |
| Default Go tests | `internal/tasktestdb/`, [CONTRIBUTING.md](CONTRIBUTING.md#before-you-open-a-pr) |
| Env or app settings | [docs/configuration.md](docs/configuration.md), Settings SPA |

## Tooling and rules

- **Plan mode:** `.cursor/rules/plan-mode.mdc` — agent asks **single plan vs parent + child plans** before `CreatePlan`; see umbrella + child layout under `.cursor/plans/`
- **Cursor rules:** `CODE_STANDARDS.mdc`, `codebase_comments.mdc`, `backend-engineering-bar.mdc`, `frontend_bar.mdc`, `plan-mode.mdc` (plan mode only)
- **CI:** `go-lint` runs `./scripts/check-go.sh --lint-only --verbose`; `go-tests` matrix runs `./scripts/check-go.sh --tests-only --group=<name> --verbose`; web job runs `./scripts/check-web.sh --install --verbose` — see `.github/workflows/ci.yml`
- **Local bar:** see [CONTRIBUTING.md § Before you open a PR](CONTRIBUTING.md#before-you-open-a-pr)
- **TDD default:** failing test first, then implement until green

## Commands to run before you finish

**Use the local check script** — it is the same bar CI runs (`check-go.sh` + `check-web.sh`). Do **not** poll GitHub Actions (`gh run watch`) as verification; run check locally and fix until green.

| Platform | Command |
| --- | --- |
| Unix / Git Bash | `./scripts/check.sh` (add `--install` when `web/package-lock.json` changed) |
| Windows PowerShell | `.\scripts\check.ps1` (add `-Install` when lockfile changed) |
| Docker only (no local Go/Node) | `docker compose run --rm dev ./scripts/check.sh --install` |

Scoped: `--go-only` / `-GoOnly`, `--web-only` / `-WebOnly`. See [CONTRIBUTING.md § Before you open a PR](CONTRIBUTING.md#before-you-open-a-pr) for CI-parity flags (`--verbose`, etc.).

| Change | If not running full check |
| --- | --- |
| Go production code or tests | `go vet ./...`, then `go test ./... -count=1`; format touched `*.go` with `gofmt`. |
| Reproduce a failing CI test group | `./scripts/check-go.sh --tests-only --group=<core\|tasks\|agents\|harness> --verbose` |
| Meaningful `web/` change | `cd web && npm test -- --run && npm run lint && npm run check:standards && npm run build` |

Default tests must not require real Postgres, real outbound network, or a running `taskapi` (see [CONTRIBUTING.md](CONTRIBUTING.md#before-you-open-a-pr) and `backend-engineering-bar.mdc` §11).

## Conventions worth remembering

- New tasks API: domain → store → handler → optional `web/` ([pkgs/tasks/handler/README.md](pkgs/tasks/handler/README.md), [docs/domain/persistence.md](docs/domain/persistence.md)).
- JSON at the boundary: web treats responses as `unknown` until `parseTaskApi` validates.
- Same-origin in prod: no CORS on `taskapi`; dev uses Vite proxy (`web/vite.config.ts`).
- Docs: update the focused doc when behavior changes — [docs/README.md](docs/README.md) is the index.

## Quick pitfalls

- Do not add `fetch` to `web/src` components — use `web/src/api/`.
- Do not rely on `taskapi` serving `web/dist`; production is static files + reverse proxy.
- `GET /events` is SSE; `/health` is plain JSON — different clients.
- Default per-IP rate limit is 120/min (`HAMIX_RATE_LIMIT_PER_MIN`); set **`0`** to disable locally.
- Verify with **`./scripts/check.sh`** / **`.\scripts\check.ps1`** — not **`gh run watch`** (see `.cursor/rules/verification.mdc`).

## Full indexes

- **Docs (read when):** [docs/README.md](docs/README.md)
- **Code paths:** [docs/agent-map.md](docs/agent-map.md)
