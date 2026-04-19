# Troubleshooting

Common issues when running `taskapi`, `web/`, and dev scripts.

## Full reload on `/tasks/<id>` shows raw JSON (not the React app)

Cause: In dev, Vite proxies `/tasks` to `taskapi`. A full page navigation to `http://localhost:5173/tasks/<uuid>` must still load the SPA.

Fix: Use a current `web/vite.config.ts` (it bypasses the proxy when `Accept` includes `text/html` so `index.html` is served). Restart `npm run dev` after pulling.

## SSE “Connected” but the Updates timeline does not grow

Cause: Without the dev ticker, `task_updated` SSE only fires after real writes. The timeline rows come from `GET /tasks/{id}/events`, not from parsing SSE bodies.

Fix: For synthetic activity in dev, set `T2A_SSE_TEST=1` in `.env` and restart `taskapi` (see [docs/API-SSE.md](./API-SSE.md)). Use `T2A_SSE_TEST_EVENTS_PER_TICK` for faster timeline churn, `T2A_SSE_TEST_SYNC_ROW=1` so task headers match synthetic audit rows, and `T2A_SSE_TEST_LIFECYCLE=1` for `task_created` / `task_deleted` hints. Ensure React Query refetches (the app invalidates on SSE `onmessage`).

## `No repository is configured for file search` in the rich prompt

Cause: `app_settings.repo_root` is empty (the workspace repo has not been configured on the SPA Settings page).

Fix: Open the SPA, click the gear icon in the header, and set the **Workspace repository** to an absolute path to the workspace you want `@` mentions to search. The supervisor reloads in-process; no `taskapi` restart is needed. See [SETTINGS.md](./SETTINGS.md).

## Web cannot reach the API (errors on fetch / EventSource)

Cause: `taskapi` not running, wrong port, or Vite proxy target mismatch.

Fix: Default API is `http://127.0.0.1:8080`. If you change the API port, set `VITE_TASKAPI_ORIGIN` for the Vite dev server (and `DEV_TASKAPI_PORT` for `scripts/dev.*` if you use them). See root [README.md](../README.md).

## Matching a failing request to logs (request id and build version)

When the UI or `curl` shows an error, you can tie it to JSONL and confirm which binary handled traffic without a separate analytics stack.

- **Request id:** Task API JSON errors may include **`request_id`** in the body (and the response may echo **`X-Request-ID`**). The same value appears on structured **`http.access`** lines and related handler logs when access middleware ran — see [API-HTTP.md](./API-HTTP.md) (headers + errors) and [WEB.md](./WEB.md) (`readError` appends the id for thrown errors).
- **Build version:** **`GET /health`**, **`/health/live`**, and **`/health/ready`** return JSON **`version`**. `taskapi` logs that same string on the **`listening`** line (`operation` **`taskapi.serve`**); **`dbcheck`** logs **`version`** on **`dbcheck.start`**. Details: [OBSERVABILITY.md](./OBSERVABILITY.md) (build identity row).

## Tests fail with “database” or connection errors

Cause: Default `go test ./...` should use SQLite test helpers, not real Postgres.

Fix: If you added tests that need `DATABASE_URL`, gate them behind `//go:build integration` or use `tasktestdb.OpenSQLite` like existing store/handler tests.

## `gofmt` / `go vet` / `npm run build` failures in CI

Cause: Local tree diverged from `main` or dependencies not installed.

Fix: Run `(cd web && npm ci)` and `./scripts/check.sh` locally; fix `gofmt` with `go fmt ./...` or `gofmt -w` on listed files.

## Local checks and agent test failures (quick playbook)

Use this in order when `go test`, `go vet`, or `web/` tests fail locally or in CI.

1. **Re-run without cache:** `go test ./... -count=1` from the repo root (matches `.cursor/rules/09-local-verification.mdc` and CI).
2. **Flaky or env-related Go tests:** Do not use `t.Parallel()` together with `t.Setenv` / `t.Chdir` in the same test (see `.cursor/rules/06-testing.mdc`). Split or serialize those tests.
3. **Database errors in default tests:** Default tests must use SQLite helpers (e.g. `tasktestdb.OpenSQLite`), not real Postgres. If a test needs `DATABASE_URL`, gate it with `//go:build integration` or refactor to in-memory SQLite (see **Tests fail with “database” or connection errors** above).
4. **Web failures:** From `web/`, run `npm ci` then `npm test -- --run` then `npm run build`. Clear stale installs if versions drift (`rm -rf node_modules` on Unix, or delete `web/node_modules` on Windows, then `npm ci`).
5. **Still stuck:** Compare with CI (`.github/workflows/ci.yml`) and run the same full bar as [CONTRIBUTING.md](../CONTRIBUTING.md): `./scripts/check.sh` or `.\scripts\check.ps1`.
