# Troubleshooting

Common issues when running `taskapi`, `web/`, and dev scripts.

## Full reload on `/tasks/<id>` shows raw JSON (not the React app)

Cause: In dev, Vite proxies `/tasks` to `taskapi`. A full page navigation to `http://localhost:5173/tasks/<uuid>` must still load the SPA.

Fix: Use a current `web/vite.config.ts` (it bypasses the proxy when `Accept` includes `text/html` so `index.html` is served). Restart `npm run dev` after pulling.

## SSE “Connected” but the Updates timeline does not grow

Cause: Without the dev ticker, `task_updated` SSE only fires after real writes. The timeline rows come from `GET /tasks/{id}/events`, not from parsing SSE bodies.

Fix: For synthetic activity in dev, set `T2A_SSE_TEST=1` in `.env` and restart `taskapi` (see [docs/DESIGN.md](./DESIGN.md)). Ensure React Query refetches (the app invalidates on SSE `onmessage`).

## `No repository is configured for file search` in the rich prompt

Cause: `REPO_ROOT` is unset or empty on `taskapi`.

Fix: Set `REPO_ROOT` to an absolute path to the workspace you want `@` mentions to search, in `.env`, and restart `taskapi`.

## Web cannot reach the API (errors on fetch / EventSource)

Cause: `taskapi` not running, wrong port, or Vite proxy target mismatch.

Fix: Default API is `http://127.0.0.1:8080`. If you change the API port, set `VITE_TASKAPI_ORIGIN` for the Vite dev server (and `DEV_TASKAPI_PORT` for `scripts/dev.*` if you use them). See root [README.md](../README.md).

## Tests fail with “database” or connection errors

Cause: Default `go test ./...` should use SQLite test helpers, not real Postgres.

Fix: If you added tests that need `DATABASE_URL`, gate them behind `//go:build integration` or use `testdb.OpenSQLite` like existing store/handler tests.

## `gofmt` / `go vet` / `npm run build` failures in CI

Cause: Local tree diverged from `main` or dependencies not installed.

Fix: Run `(cd web && npm ci)` and `./scripts/check.sh` locally; fix `gofmt` with `go fmt ./...` or `gofmt -w` on listed files.
