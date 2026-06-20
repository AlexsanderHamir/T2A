# Contributing to T2A

Thanks for helping improve the project. This file is the short path for humans and agents; deeper contracts live in `docs/`.

## Security

For **undisclosed vulnerabilities**, use [SECURITY.md](SECURITY.md) (private advisory on GitHub, not a public issue). Dependency update PRs may be opened by [Dependabot](.github/dependabot.yml); review and run `./scripts/check.sh` (or `.\scripts\check.ps1`) before merging.

## Before you start

1. Read [AGENTS.md](AGENTS.md) (repo map, commands, pitfalls).
2. Copy [.env.example](.env.example) to `.env` and set `DATABASE_URL`. The workspace repo path, agent worker switches, cursor binary, and per-run timeout live in the SPA Settings page (gear icon in the header → `/settings`); see [docs/configuration.md](docs/configuration.md). Never commit `.env`.
3. Authoritative HTTP/SSE/JSON behavior: [docs/api.md](docs/api.md); architecture and limitations: [docs/architecture.md](docs/architecture.md); env + app settings: [docs/configuration.md](docs/configuration.md); optional UI client: [docs/web.md](docs/web.md).

## Local setup

- Go 1.25+ and Node 20+ (for `web/`; `web/package.json` **`engines.node`** matches CI).
- Migrate/check DB: `go run ./cmd/dbcheck -migrate` (see root [README.md](README.md)).
- API only: `go run ./cmd/taskapi`
- API + Vite together: `scripts/dev.ps1` or `scripts/dev.sh` from the repo root.

## Before opening a PR

From the repo root, run the full bar from [AGENTS.md](AGENTS.md#commands-to-run-before-you-finish) (covers what CI enforces across the **backend** and **web** jobs in `.github/workflows/ci.yml`):

```bash
(cd web && npm ci)   # first time or after lockfile changes
./scripts/check.sh
```

Windows: `.\scripts\check.ps1` (install `web/` deps with `npm ci` in `web/` when needed). Go-only quick path: `CHECK_SKIP_WEB=1 ./scripts/check.sh`.

**Tests:** Prefer **test-first** for bugs and new behavior (failing test → fix → green); Go patterns in `.cursor/rules/backend-engineering-bar.mdc` §11, web patterns in [docs/web.md](docs/web.md) and co-located `*.test.tsx` files. For **`pkgs/tasks/middleware`**, put exported-API-only tests in **`internal/middlewaretest/`** and keep whitebox tests next to the implementation (see `pkgs/tasks/middleware/README.md` § Tests). For **`pkgs/tasks/handler`** growth and where to put new tests vs extractions, see [docs/contributing.md](docs/contributing.md).

**Observability:** Run `./scripts/measure-func-slog.sh` (or `.\scripts\measure-func-slog.ps1`) for the per-function `slog` audit, and `./scripts/measure-observability.sh` (or `.\scripts\measure-observability.ps1`) if you need test coverage numbers.

## Changing APIs or JSON

When you change REST paths, query params, response shapes, SSE payload types, or audit event types:

- Update [docs/api.md](docs/api.md) and [docs/architecture.md](docs/architecture.md) when limitations change. Update [docs/configuration.md](docs/configuration.md) when env vars or app settings change.
- Reorder or add `With*` middleware for `taskapi`: edit `pkgs/tasks/middleware.Stack` (used by `internal/taskapi.NewHTTPHandler` with `calltrace.Path`).
- Update `web/src/api/parseTaskApi.ts` (and `web/src/types/` if needed) and tests.
- Update Go handler/store tests so defaults still pass without real Postgres or network.

## Adding features (layering)

Prefer a vertical slice: `domain` types and validation → `store` use-case methods → `handler` decode/map errors/`notifyChange` → optional `web/src/api` + UI. Human summary and checklist: [docs/contributing.md](docs/contributing.md).

For **task UI** under `web/src/tasks/`, keep new pieces in the right family folder and import through its `index.ts` barrel where one exists; conventions are summarized under **`tasks/components/` layout** in [docs/web.md](docs/web.md).

## Cursor / AI rules

Rules under `.cursor/rules/` cover structure (`CODE_STANDARDS.mdc`), comments (`codebase_comments.mdc`), Go quality (`backend-engineering-bar.mdc`), and UI quality (`frontend_bar.mdc`).

**Full pass:** For cross-cutting or high-risk changes, run the same local bar as CI with `.\scripts\check.ps1` or `./scripts/check.sh`. Narrow only when all touched files fit Go-only, `web/`-only, or docs-only scope.

**Default agent behavior:** If scope is unspecified, assume **full repo**; narrow only when the touched files and user request clearly fit a smaller scope.

## Stuck?

See [docs/contributing.md](docs/contributing.md#troubleshooting) for common dev issues (Vite proxy, SSE dev mode, refreshes).
