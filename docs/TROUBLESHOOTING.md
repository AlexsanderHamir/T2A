# Troubleshooting

Common issues when running `taskapi`, `web/`, and dev scripts.

## Full reload on `/tasks/<id>` shows raw JSON (not the React app)

Cause: In dev, Vite proxies `/tasks` to `taskapi`. A full page navigation to `http://localhost:5173/tasks/<uuid>` must still load the SPA.

Fix: Use a current `web/vite.config.ts` (it bypasses the proxy when `Accept` includes `text/html` so `index.html` is served). Restart `npm run dev` after pulling.

## SSE “Connected” but the Updates timeline does not grow

Cause: Without the dev ticker, `task_updated` SSE only fires after real writes. The timeline rows come from `GET /tasks/{id}/events`, not from parsing SSE bodies.

Fix: For synthetic activity in dev, set `T2A_SSE_TEST=1` in `.env` and restart `taskapi` (see [docs/DESIGN.md](./DESIGN.md)). Use `T2A_SSE_TEST_EVENTS_PER_TICK` for faster timeline churn, `T2A_SSE_TEST_SYNC_ROW=1` so task headers match synthetic audit rows, and `T2A_SSE_TEST_LIFECYCLE=1` for `task_created` / `task_deleted` hints. Ensure React Query refetches (the app invalidates on SSE `onmessage`).

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

## Local checks and agent test failures (quick playbook)

Use this in order when `go test`, `go vet`, or `web/` tests fail locally or in CI.

1. **Re-run without cache:** `go test ./... -count=1` from the repo root (matches `.cursor/rules/09-local-verification.mdc` and CI).
2. **Flaky or env-related Go tests:** Do not use `t.Parallel()` together with `t.Setenv` / `t.Chdir` in the same test (see `.cursor/rules/06-testing.mdc`). Split or serialize those tests.
3. **Database errors in default tests:** Default tests must use SQLite helpers (e.g. `testdb.OpenSQLite`), not real Postgres. If a test needs `DATABASE_URL`, gate it with `//go:build integration` or refactor to in-memory SQLite (see **Tests fail with “database” or connection errors** above).
4. **Web failures:** From `web/`, run `npm ci` then `npm test -- --run` then `npm run build`. Clear stale installs if versions drift (`rm -rf node_modules` on Unix, or delete `web/node_modules` on Windows, then `npm ci`).
5. **Still stuck:** Compare with CI (`.github/workflows/ci.yml`) and run the same full bar as [CONTRIBUTING.md](../CONTRIBUTING.md): `./scripts/check.sh` or `.\scripts\check.ps1`.
