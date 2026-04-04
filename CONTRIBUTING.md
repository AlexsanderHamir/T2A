# Contributing to T2A

Thanks for helping improve the project. This file is the short path for humans and agents; deeper contracts live in `docs/`.

## Before you start

1. Read [AGENTS.md](AGENTS.md) (repo map, commands, pitfalls).
2. Copy [.env.example](.env.example) to `.env` and set `DATABASE_URL` (and optionally `REPO_ROOT`). Never commit `.env`.
3. Authoritative HTTP/SSE/JSON behavior: [docs/DESIGN.md](docs/DESIGN.md) · optional UI client: [docs/WEB.md](docs/WEB.md).

## Local setup

- Go 1.25+ and Node 20+ (for `web/`).
- Migrate/check DB: `go run ./cmd/dbcheck -migrate` (see root [README.md](README.md)).
- API only: `go run ./cmd/taskapi`
- API + Vite together: `scripts/dev.ps1` or `scripts/dev.sh` from the repo root.

## Before opening a PR

From the repo root, run the full bar (same as CI):

```bash
(cd web && npm ci)   # first time or after lockfile changes
./scripts/check.sh
```

Windows: `.\scripts\check.ps1` (install `web/` deps with `npm ci` in `web/` when needed).

Go-only quick path: `CHECK_SKIP_WEB=1 ./scripts/check.sh`.

**Tests:** Prefer **test-first** for bugs and new behavior (failing test → fix → green); details in `.cursor/rules/06-testing.mdc` (Go) and `.cursor/rules/10-web-ui.mdc` (`web/`).

## Changing APIs or JSON

When you change REST paths, query params, response shapes, SSE payload types, or audit event types:

- Update `docs/DESIGN.md` (and `README.md` / `docs/WEB.md` if user-facing commands or Vite env change).
- Update `web/src/api/parseTaskApi.ts` (and `web/src/types/` if needed) and tests.
- Update Go handler/store tests so defaults still pass without real Postgres or network.

See `.cursor/rules/11-api-contracts.mdc` for a compact checklist.

## Adding features (layering)

Prefer a vertical slice: `domain` types and validation → `store` use-case methods → `handler` decode/map errors/`notifyChange` → optional `web/src/api` + UI. Full checklist: `.cursor/rules/13-tasks-stack-extensibility.mdc`. Human summary: `docs/DESIGN.md` (section Extensibility).

## Cursor / AI rules

Numbered rules under `.cursor/rules/` cover style, tests, security, web UI, documentation prose (`12-documentation-style.mdc`), tasks stack extensibility (`13-tasks-stack-extensibility.mdc`), workspace repo extensibility (`14-repo-workspace-extensibility.mdc`), and database schema / AutoMigrate (`15-database-schema.mdc`).

**Deterministic full pass:** In Cursor chat, **@-mention** [`.cursor/rules/00-full-rules-pass.mdc`](.cursor/rules/00-full-rules-pass.mdc) (or ask explicitly to “run the full rules pass”) for cross-cutting or high-risk changes. That playbook defines default scope (**full repo** unless narrowed), when **docs-and-rules-only** skips `go test` / `npm`, and the completion checklist.

**Default agent behavior:** If scope is unspecified, assume **full repo**; narrow only when all touched files fit Go-only, `web/`-only, or docs-and-rules-only (see `00-full-rules-pass.mdc` and `99-repo-primer.mdc`).

## Stuck?

See [docs/TROUBLESHOOTING.md](docs/TROUBLESHOOTING.md) for common dev issues (Vite proxy, SSE dev mode, refreshes).
