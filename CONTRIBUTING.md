# Contributing to T2A

Thanks for helping improve the project. This file is the short path for humans and agents; deeper contracts live in **`docs/`**.

## Before you start

1. Read **[`AGENTS.md`](AGENTS.md)** (repo map, commands, pitfalls).
2. Copy **[`.env.example`](.env.example)** to **`.env`** and set **`DATABASE_URL`** (and optionally **`REPO_ROOT`**). Never commit **`.env`**.
3. Authoritative HTTP/SSE/JSON behavior: **[`docs/DESIGN.md`](docs/DESIGN.md)** · optional UI client: **[`docs/WEB.md`](docs/WEB.md)**.

## Local setup

- **Go** 1.25+ and **Node** 20+ (for **`web/`**).
- Migrate/check DB: `go run ./cmd/dbcheck -migrate` (see root **`README.md`**).
- API only: `go run ./cmd/taskapi`
- API + Vite together: **`scripts/dev.ps1`** or **`scripts/dev.sh`** from the repo root.

## Before opening a PR

From the repo root, run the full bar (same as CI):

```bash
(cd web && npm ci)   # first time or after lockfile changes
./scripts/check.sh
```

Windows: **`.\scripts\check.ps1`** (install **`web/`** deps with **`npm ci`** in **`web/`** when needed).

Go-only quick path: **`CHECK_SKIP_WEB=1 ./scripts/check.sh`**.

## Changing APIs or JSON

When you change REST paths, query params, response shapes, SSE payload types, or audit event types:

- Update **`docs/DESIGN.md`** (and **`README.md`** / **`docs/WEB.md`** if user-facing commands or Vite env change).
- Update **`web/src/api/parseTaskApi.ts`** (and **`web/src/types/`** if needed) and tests.
- Update Go handler/store tests so defaults still pass without real Postgres or network.

See **`.cursor/rules/11-api-contracts.mdc`** for a compact checklist.

## Cursor / AI rules

Numbered rules under **`.cursor/rules/`** cover style, tests, security, and web UI. **`00-full-rules-pass.mdc`** describes how to run a full pass when asked.

## Stuck?

See **[`docs/TROUBLESHOOTING.md`](docs/TROUBLESHOOTING.md)** for common dev issues (Vite proxy, SSE dev mode, refreshes).
