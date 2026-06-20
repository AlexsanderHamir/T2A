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
| Changing Go REST / handlers | [docs/api.md](docs/api.md), [docs/contributing.md](docs/contributing.md) §Splitting handler | harness docs, [docs/web.md](docs/web.md) |
| Changing Go domain / store | [docs/data-model.md](docs/data-model.md), [pkgs/tasks/store/README.md](pkgs/tasks/store/README.md) | web, harness |
| Changing agent worker / harness | [docs/domain/harness.md](docs/domain/harness.md), [docs/configuration.md](docs/configuration.md) | [docs/web.md](docs/web.md) |
| Changing web UI only | [docs/web.md](docs/web.md), `.cursor/rules/frontend_bar.mdc` | architecture, harness |
| Changing web data (API / sync / mutations) | [docs/web.md](docs/web.md) §Task sync / Query policy, `web/src/api/` | handler split guide |
| Adding a full-stack feature | [docs/contributing.md](docs/contributing.md) §Adding a feature, [docs/api.md](docs/api.md) | — |
| Writing operator / checklist copy | [docs/execute-and-verify.md](docs/execute-and-verify.md) | code map |
| Docs or config only | Target doc from [docs/README.md](docs/README.md) | code map |

## Where to find X

| I need to… | Go to |
| --- | --- |
| Add or change a REST route | `pkgs/tasks/handler/handler_*.go`, [docs/api.md](docs/api.md) |
| Add DB persistence | `pkgs/tasks/store/`, [docs/domain/persistence.md](docs/domain/persistence.md) |
| Change task JSON shape | `pkgs/tasks/domain/`, `handler_*_json.go`, `web/src/api/parseTaskApi*.ts` |
| Wire SSE after a write | handler `notifyChange`, [docs/domain/sse-hub.md](docs/domain/sse-hub.md) |
| Fix live UI not updating | `web/src/tasks/sync/`, [docs/web.md](docs/web.md) §Task sync |
| Add a fetch call | `web/src/api/` only — never components |
| Add a page or route | `web/src/app/Router.tsx`, `web/src/tasks/pages/` |
| Task templates UI or API | `web/src/api/taskTemplates.ts`, `TaskTemplatesPage.tsx`, `handler_task_templates*.go` |
| Create or edit task modal | `web/src/tasks/create/`, `task-create-modal/` |
| Execution cycles UI | `web/src/tasks/components/task-detail/` (cycles panel) |
| Agent run / verify loop | `pkgs/agents/harness/`, [docs/domain/harness.md](docs/domain/harness.md) |
| Worker queue / pickup | `pkgs/agents/worker/`, [docs/domain/agent-queue.md](docs/domain/agent-queue.md) |
| Env or app settings | [docs/configuration.md](docs/configuration.md), Settings SPA |
| Default Go tests | `internal/tasktestdb/`, [docs/contributing.md](docs/contributing.md) §Tests |
| Middleware change | `pkgs/tasks/middleware/`, `internal/middlewaretest/` |
| Where a new file goes | `.cursor/rules/CODE_STANDARDS.mdc` Part 12 |
| Handler file too large | [docs/contributing.md](docs/contributing.md) §Splitting handler |
| UI tokens or spacing | `web/src/app/styles/tokens/`, `frontend_bar.mdc` |
| Hidden launch features | [docs/omitted-features.md](docs/omitted-features.md) |
| Local dev / install | [README.md](README.md) |
| PR checklist | [CONTRIBUTING.md](CONTRIBUTING.md) |
| Test failure triage | [docs/contributing.md](docs/contributing.md) §Troubleshooting |

## Tooling and rules

- **Cursor rules:** `CODE_STANDARDS.mdc`, `codebase_comments.mdc`, `backend-engineering-bar.mdc`, `frontend_bar.mdc`
- **CI:** backend job (`gofmt`, `go vet`, `go test`, `funclogmeasure -enforce`); web job (`npm test`, `lint`, `check:standards`, `build`) — see `.github/workflows/ci.yml`
- **Local bar:** `./scripts/check.sh` or `.\scripts\check.ps1`; Go-only: `CHECK_SKIP_WEB=1`
- **TDD default:** failing test first, then implement until green

## Commands to run before you finish

| Change | Command |
| --- | --- |
| Full bar (recommended) | `.\scripts\check.ps1` (Windows) or `./scripts/check.sh` (Unix). Go-only: `CHECK_SKIP_WEB=1`. Skip funclogmeasure locally: `CHECK_SKIP_FUNCLOG=1`. |
| Go production code or tests | `go vet ./...`, then `go test ./... -count=1`; format touched `*.go` with `gofmt`. |
| Meaningful `web/` change | `cd web && npm test -- --run && npm run lint && npm run check:standards && npm run build` |

Default tests must not require real Postgres, real outbound network, or a running `taskapi` (see [docs/contributing.md](docs/contributing.md) §Tests and `backend-engineering-bar.mdc` §11).

## Conventions worth remembering

- New tasks API: domain → store → handler → optional `web/` ([docs/contributing.md](docs/contributing.md)).
- JSON at the boundary: web treats responses as `unknown` until `parseTaskApi` validates.
- Same-origin in prod: no CORS on `taskapi`; dev uses Vite proxy (`web/vite.config.ts`).
- Docs: update the focused doc when behavior changes — [docs/README.md](docs/README.md) is the index.

## Quick pitfalls

- Do not add `fetch` to `web/src` components — use `web/src/api/`.
- Do not rely on `taskapi` serving `web/dist`; production is static files + reverse proxy.
- `GET /events` is SSE; `/health` is plain JSON — different clients.
- Default per-IP rate limit is 120/min (`T2A_RATE_LIMIT_PER_MIN`); set **`0`** to disable locally.

## Full indexes

- **Docs (read when):** [docs/README.md](docs/README.md)
- **Code paths:** [docs/agent-map.md](docs/agent-map.md)
