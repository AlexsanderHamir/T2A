# Agent orientation (AI + contributors)

Use this file as the first pass before editing code. Long-form contracts live in `docs/`; this file is a map and checklist.

## Read order

| Order | Doc | Why |
|------|-----|-----|
| 1 | [README.md](README.md) | Install, run `taskapi` / `dbcheck`, `web/` npm commands, dev scripts. |
| 2 | [CONTRIBUTING.md](CONTRIBUTING.md) | PR checklist, `.env.example`, API/doc sync pointers. |
| 3 | [docs/DESIGN.md](docs/DESIGN.md) | HTTP routes, SSE, env vars (`DATABASE_URL`, `REPO_ROOT`), persistence, limitations. |
| 4 | [docs/WEB.md](docs/WEB.md) | `web/src` layout, React Query + SSE, `parseTaskApi`, Vitest. |
| 5 | [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) | Common dev failures (Vite proxy, SSE, `REPO_ROOT`). |

Cursor: `99-repo-primer.mdc` (always-on), `01`–`08`, `11-api-contracts` (HTTP/JSON sync), `13-tasks-stack-extensibility` (tasks API layering), `14-repo-workspace-extensibility` (`REPO_ROOT` / `/repo` / `pkgs/repo`), `12-documentation-style` (README/docs prose), `09-local-verification` + `09-security-baseline`, `10-web-ui` for `web/`. `00-full-rules-pass.mdc` defines scope (full repo vs Go-only vs web-only vs frontend-then-backend vs audit-only), phases, and the completion report. `06-testing.mdc` defines `go test` expectations; `10-web-ui.mdc` defines `npm test` for `web/`. CI runs `scripts/check.sh` on push/PR (`.github/workflows/ci.yml`).

## Repository map

| Area | Path | Notes |
|------|------|--------|
| HTTP API + SSE | `pkgs/tasks/handler/` | REST `/tasks`, `GET /events`, `/repo/*` when `REPO_ROOT` set; `GET /health`. |
| Persistence | `pkgs/tasks/store/`, `pkgs/tasks/postgres/` | Store maps DB errors to `domain.ErrNotFound` / `ErrInvalidInput`. |
| Domain types | `pkgs/tasks/domain/` | Status, priority, task model, audit events. |
| Workspace search | `pkgs/repo/` | Optional; used for `@file` mentions when repo configured. |
| Env loading | `internal/envload/` | Resolves `.env` from repo root. |
| Dev UI simulation | `internal/devsim/` | Optional `T2A_SSE_TEST` ticker: synthetic audit events + SSE hints (`cmd/taskapi` wires `store` + hub). |
| Binaries | `cmd/taskapi/`, `cmd/dbcheck/` | Entry points only. |
| Web SPA | `web/` | Vite + React; `fetch` only under `web/src/api/`; import `@/types`, `@/api`. |

API contracts (paths, query params, JSON shapes) are authoritative in `docs/DESIGN.md` and `docs/WEB.md`, not only in prose comments.

## Commands to run before you finish

| Change | Command |
|--------|---------|
| Full bar (recommended) | From repo root: `.\scripts\check.ps1` (Windows) or `./scripts/check.sh` (Unix). Go-only fast path: set `CHECK_SKIP_WEB=1` (bash) or `$env:CHECK_SKIP_WEB='1'` (PowerShell) to skip `web/` steps. |
| Go production code or tests | `go vet ./...`, then `go test ./... -count=1` (from repo root); format touched `*.go` with `gofmt` or `go fmt`. |
| Meaningful `web/` change | `cd web && npm test -- --run && npm run build` |
| Coverage / quality (Go libs) | See `.cursor/rules/06-testing.mdc` (`coverprofile` on `pkgs/...` `internal/...`) |

Default tests must not require real Postgres, real outbound network, or a running `taskapi` (see `06-testing.mdc` and `10-web-ui.mdc`).

## Conventions worth remembering

- New tasks API features: follow `docs/DESIGN.md` section Extensibility (domain → store → handler → optional `web/`). Rule `.cursor/rules/13-tasks-stack-extensibility.mdc` expands the same slice for agents.
- JSON at the boundary: Web treats responses as `unknown` until `parseTaskApi` validates; keep that pipeline when adding fields.
- Same-origin in prod: `taskapi` does not add CORS; dev uses Vite proxy (`web/vite.config.ts`).
- Atomic commits: `.cursor/rules/08-atomic-commits.mdc` — one logical concern per commit, conventional message style; push after committing unless the user opts out or push is not possible.
- Docs: When you change flags, routes, or env vars, update `docs/DESIGN.md` (and `docs/WEB.md` / root `README.md` if user-facing commands change); see `docs/README.md` “Where to put updates”.

## Quick pitfalls

- Do not add `fetch` to `web/src` components for app APIs — use `web/src/api/`.
- Do not rely on `taskapi` serving `web/dist`; production is static files + reverse proxy or same-origin gateway.
- `GET /events` is SSE; `/health` is plain JSON — different clients.
