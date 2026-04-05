# Observability standard (T2A)

This document defines **how we measure** logging and correlation in `taskapi`, and **how we increase** test coverage when we change the API or background behavior. It complements [DESIGN.md](./DESIGN.md) (what the server does) and `.cursor/rules/04-structured-logging.mdc` (log shape and safety).

## Goals

### Runtime (operators)

From production logs (and future metrics if we add them), we want to answer:

1. **Which request failed or was slow?** (`request_id`, `operation`, `http_status`, `duration_ms`, SQL `trace.duration`).
2. **What code path ran?** Stable `operation` values and route/method on the access line.
3. **Background work** (dev ticker, etc.) is clearly **not** tied to a request id so we do not confuse it with user traffic.

**Volume:** `taskapi` writes JSON lines at a configurable **minimum level**: **`-loglevel`** or **`T2A_LOG_LEVEL`** (`debug`, `info`, `warn`, `error`; default **`info`** for lighter production logs). Set **`debug`** for full trace lines. At **`info`**, `slog.Debug` records are dropped; `Info`+ still go to the file. **`-disable-logging`** or **`T2A_DISABLE_LOGGING`** (`1`/`true`/`yes`/`on`) skips the file and emits **only `slog.Error`** to stderr. See **`docs/DESIGN.md`** (startup flow and env table).

### Codebase (static “at least one log per function”)

**Target:** every **named** function or method in production `.go` files (excluding generated sources) should contain **at least one type-resolved call** into **`log/slog`** in its body. Using **`slog.LevelDebug`** is fine for noisy or trivial paths.

**Measure:** `funclogmeasure` (`go run ./cmd/funclogmeasure` or `scripts/measure-func-slog.sh` / `scripts/measure-func-slog.ps1`). It loads **`./...`** with **`go/types`** (via `golang.org/x/tools/go/packages`) and counts any call that resolves to a **`log/slog`** function or method (for example `slog.Info`, `slog.Default().Info`, `logger.Info` when `logger` is a `*slog.Logger`, and dot-imported `Info`).

**Caveats (read before `-enforce`):**

- Calls are matched by **type information**, not text; non-`slog` loggers or `interface{ Info(...) }` calls are **not** counted.
- **Nested function literals** are **not** walked; only the outer named function’s body counts. The outer still needs its own `slog` call if you want it to pass.
- Functions in files with **no** successful type check for their package may be skipped (a warning is logged to stderr).
- Helpers that **only** call non-`slog` wrappers (for example pure `writeStoreError` without a `slog` call inside **this** function) do **not** count.
- **`cmd/funclogmeasure`** is **skipped by default**; use `-include-tool` to audit it too.

We do **not** treat a single percentage as a product SLO. Use the **checklists** below, **`funclogmeasure`** for the per-function log target, and **test coverage** scripts where they still help.

## Signals (today)

| Signal | Role in T2A | Standard |
|--------|----------------|----------|
| **Structured logs** | Primary signal: JSON lines per process run | `slog` with stable keys; errors include `err`; no secrets (see security baseline rule). |
| **Request correlation** | Tie access line, handler errors, and GORM SQL | `request_id` on the request context; response header **`X-Request-ID`** (client echo or server UUID). Health/readiness paths omit the **`http.access`** line but still set the header and context id. |
| **Build identity** | Match JSONL / CLI to binary and health probes | **`internal/version.String()`** (module tag, short **`vcs.revision`**, **`devel`**, or **`unknown`**). `taskapi` logs **`version`** on **`listening`** (`operation` **`taskapi.serve`**); **`dbcheck`** logs **`version`** and **`timeout_sec`** (connect + optional migrate context) on **`dbcheck.start`**, and **`dbcheck.done`** (**`migrate_ran`**) on success; on failure, **`dbcheck failed`** at **Error** with **`operation`** **`dbcheck.failed`** includes **`deadline_exceeded`** when the error chain is **`context.DeadlineExceeded`** (same wall clock as **`timeout_sec`**). Health JSON uses the same **`version`** via **`handler.ServerVersion()`**. |
| **Startup config** | Confirm DB pool, slow-SQL threshold, HTTP bounds, and middleware knobs (no secrets) | Right after the JSON **`slog`** handler is installed (and not in minimized logging mode), **`taskapi.logging`** records **`min_level`** and **`json_file`** **`true`**; the record’s **`level`** matches **`min_level`** so it is never dropped by the handler floor. After migrate: **`taskapi.db_config`** (pool caps from **`pkgs/tasks/postgres`**, effective **`gorm_slow_query_ms`**) and **`taskapi.http_limits`** (read/header/idle caps, **`shutdown_timeout_sec`**, **`write_timeout_disabled`** **`true`** — **`http.Server.WriteTimeout`** left unset so **`GET /events`** SSE can stay open; see **`docs/DESIGN.md`**). Before the handler stack: **`taskapi.repo_root`** (**`enabled`**, **`path`** when **`REPO_ROOT`** is set and opens cleanly), **`taskapi.rate_limit`** (**`enabled`**, **`per_ip_per_min`** — **`0`** when **`T2A_RATE_LIMIT_PER_MIN`** disables limiting), **`taskapi.max_body`** (**`enabled`**, **`max_bytes`** — **`0`** when **`T2A_MAX_REQUEST_BODY_BYTES`** is unset/disabled), **`taskapi.idempotency`** (**`enabled`**, **`ttl_sec`**). When **`T2A_SSE_TEST`** is not **`1`**, **`taskapi.sse_dev`** logs **`sse dev config`** with **`enabled`** **`false`** (see **SSE dev mode**). Never logs **`DATABASE_URL`**. **`dbcheck`** (stderr text): after **`ping`**, **`dbcheck.db_config`** — same **`postgres.LogStartupDBConfig`** fields as **`taskapi`**, **`operation`** **`cmd`+`.db_config`**. |
| **Graceful shutdown** | Correlate signals with HTTP drain and DB teardown | On SIGINT/SIGTERM: **`shutdown signal received`** (`operation` **`taskapi.shutdown`**, **`signal`**); after successful **`Server.Shutdown`**, **`http server drained`** (`phase` **`http_done`**). After a successful **`sql.DB.Close`**, **`database pool closed`** (`operation` **`taskapi.shutdown`**, **`phase`** **`db_done`**). On failed **`Shutdown`**, the error line includes **`deadline_exceeded`** when the drain hit **`taskapi.http_limits`** **`shutdown_timeout_sec`**. Failures also use **`taskapi.db_close`** with **`err`**. |
| **SSE dev mode** | Make synthetic SSE obvious in logs | **`taskapi.sse_dev`**: when **`T2A_SSE_TEST`** is not **`1`**, **`sse dev config`** with **`enabled`** **`false`**. When enabled, either **`sse dev ticker enabled`** with **`interval`**, or **`sse dev env on, ticker off`** when the interval is **`0`** or below **1s** (includes a short **`hint`**). See **`docs/DESIGN.md`**. |
| **Log order** | Sort JSONL within a request or the process | `log_seq` (monotonic) with `log_seq_scope` `request` (access middleware) or `process` (startup, `/health`, background). |
| **Line kind** | Filter JSONL in tools | `obs_category`: `http_access`, `http_io`, `helper_io`. |
| **Access line** | One completion record per HTTP request (except `GET /health`, `/health/live`, `/health/ready`) | `operation` = `http.access`; includes `method`, `path`, `route`, `status`, `duration_ms`, `bytes_written`. |
| **Handler panic** | Rare bugs, easier triage | **`operation`** **`http.recover`** at **Error**: **`method`**, **`path`**, **`panic`**, **`stack`**; includes **`request_id`** when the request ran through access middleware. Client sees **500** JSON (**`internal server error`**). |
| **SQL traces** | DB latency and shape | GORM → same `slog` sink; parameterized SQL; statements slower than **`T2A_GORM_SLOW_QUERY_MS`** (default 200ms, `0` disables) log at **Warn** with elapsed time and SQL in the `trace` group. |
| **Metrics** | Rates, histograms, SLO dashboards | **`GET /metrics`** (Prometheus text): `taskapi_http_requests_total`, `taskapi_http_request_duration_seconds`, `taskapi_http_in_flight`, `taskapi_http_rate_limited_total`, `taskapi_http_idempotent_replay_total` (health paths excluded from HTTP latency series). Per-IP limit: **`T2A_RATE_LIMIT_PER_MIN`**. Idempotency cache TTL: **`T2A_IDEMPOTENCY_TTL`**. Restrict scrapes in production. |
| **Distributed traces** | Span graphs across services | Not in scope for single-process `taskapi` unless we adopt OpenTelemetry later. |

## Checklist: increasing observability

When you add or materially change behavior, use this list (copy into a PR description if helpful).

### HTTP handlers (`pkgs/tasks/handler`)

- [ ] **Context:** Handlers use `r.Context()` for store calls so SQL logs share `request_id`.
- [ ] **Failures:** Use `writeError` / `writeStoreError` (or `slog.Log(r.Context(), …)`) so client/server errors keep correlation.
- [ ] **SSE:** Long streams still get one access line at the end; publish path uses `slog` appropriately (see `tasks.sse.publish` at Debug when subscribers exist).
- [ ] **Operations:** New code paths use a stable, grep-friendly `operation` string (existing pattern: `tasks.*`, `repo.*`, `http.*`).
- [ ] **IO visibility:** At Debug, `http.io` lines record `phase` `in`/`out`, handler `operation`, `call_path`, and safe input/output summaries; helpers emit `helper.io` with `function` and the same `call_path` (see `calllog.go`, `docs/DESIGN.md`). Use `RunObserved` when a helper should log explicit input/output key/value pairs. New routes: `withCallRoot(r, op)` first; pass `r.Context()` into helpers that support `PushCall`—avoid secrets and unbounded payloads.

### Background work (`pkgs/tasks/devsim`, tickers, etc.)

- [ ] Logs use **Info** for lifecycle (start/stop) and **Debug** for per-tick noise unless operators need it.
- [ ] No fake `request_id`; absence of `request_id` is expected.

### Tests

- [ ] Extend `pkgs/tasks/handler/observability_test.go` (or focused tests) when you add middleware, new correlation rules, or access-log behavior.
- [ ] Run **`scripts/measure-func-slog.sh`** or **`scripts/measure-func-slog.ps1`** when you add or split functions and you are driving toward the per-function `slog` target.
- [ ] Run **`scripts/measure-observability.sh`** or **`scripts/measure-observability.ps1`** if you care about **test statement coverage** (not the same as “has a log line”).

## How we measure (repeatable)

### A. Per-function `log/slog` presence (static)

From the **repository root** (scripts `cd` to repo root automatically):

| OS | Command |
|----|---------|
| Unix | `./scripts/measure-func-slog.sh` |
| Windows | `.\scripts\measure-func-slog.ps1` |

Or: `go run ./cmd/funclogmeasure` with optional flags:

| Flag | Meaning |
|------|---------|
| `-tests` | Also scan `*_test.go` files (default is production `.go` only). |
| `-json` | JSON report on stdout (summary + violations). |
| `-enforce` | Exit code 1 if any function lacks a type-resolved `log/slog` call. |
| `-max N` | Cap printed violation lines (default 200; `0` = unlimited). |
| `-include-tool` | Include `cmd/funclogmeasure` in the scan. |

### B. Test coverage (dynamic)

| OS | Command |
|----|---------|
| Unix | `./scripts/measure-observability.sh` |
| Windows | `.\scripts\measure-observability.ps1` |

The script runs **`go test ./... -coverprofile=coverage-observability.out`** so **every Go package in the module** is included in the merged profile (including **`cmd/taskapi`**, **`cmd/dbcheck`**, `pkgs/...`, `internal/...`).

**What `go tool cover -func` shows:** only **non-test** source files (no `*_test.go`) that appear in the coverage profile—per-function **statement** coverage for production code linked into test binaries. Packages with **[no test files]** often contribute **no** rows for their own `.go` files unless something else imports them.

**Working directory:** scripts resolve the **repository root from the script file path**.

The script prints the **full** `go tool cover -func` report and the trailing **`total:`** line. That **`total:`** is **not** “every function has a log”; use **`funclogmeasure`** for that.

For **library-only** coverage without `cmd/*` (e.g. matching `.cursor/rules/06-testing.mdc` examples), run `go test` yourself with `./pkgs/... ./internal/...` and a separate coverprofile.

The profile file `coverage-observability.out` is gitignored (`coverage*.out`). Remove it after review if you like.

## When to add metrics or tracing

Add **metrics** (for example HTTP RED: rate, errors, duration histograms) when:

- You need **SLOs** or alerts that logs alone cannot support cheaply, or
- You want dashboards that aggregate thousands of requests without log volume.

Add **tracing** when:

- Multiple services or async boundaries must appear on one timeline, or
- You outgrow correlation via `request_id` alone.

When we introduce either, update this doc and `docs/DESIGN.md` so operators know how to scrape or export data.

## Related docs and rules

- [DESIGN.md](./DESIGN.md) — logging, `X-Request-ID`, access line, GORM SQL.
- [AGENTS.md](../AGENTS.md) — commands including measurement script.
- `.cursor/rules/04-structured-logging.mdc` — field names, levels, secrets.
- `.cursor/rules/09-security-baseline.mdc` — no credentials in logs.
