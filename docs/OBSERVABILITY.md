# Observability standard (T2A)

This document defines **how we measure** logging and correlation in `taskapi`, and **how we increase** test coverage when we change the API or background behavior. It complements [RUNTIME-ENV.md](./RUNTIME-ENV.md) (startup, env), [API-HTTP.md](./API-HTTP.md) (routes, health), and the [DESIGN.md](./DESIGN.md) hub, plus `.cursor/rules/04-structured-logging.mdc` (log shape and safety). **Phased observability improvements** (Prometheus, SLOs, tracing): [OBSERVABILITY-ROADMAP.md](./OBSERVABILITY-ROADMAP.md). **Starter SLIs / SLOs** (30d window, error budget framing): § **SLIs and SLOs** below.

## Goals

### Runtime (operators)

From production logs (and future metrics if we add them), we want to answer:

1. **Which request failed or was slow?** (`request_id`, `operation`, `http_status`, `duration_ms`, SQL `trace.duration`).
2. **What code path ran?** Stable `operation` values and route/method on the access line.
3. **Background work** (dev ticker, etc.) is clearly **not** tied to a request id so we do not confuse it with user traffic.

**Volume:** `taskapi` writes JSON lines at a configurable **minimum level**: **`-loglevel`** or **`T2A_LOG_LEVEL`** (`debug`, `info`, `warn`, `error`; default **`info`** for lighter production logs). Set **`debug`** for full trace lines. At **`info`**, `slog.Debug` records are dropped; `Info`+ still go to the file. **`-disable-logging`** or **`T2A_DISABLE_LOGGING`** (`1`/`true`/`yes`/`on`) skips the file and emits **only `slog.Error`** to stderr. See **`docs/RUNTIME-ENV.md`** (startup flow and env table).

### Codebase (static “at least one log per function”)

**Target:** every **named** function or method in production `.go` files (excluding generated sources) should contain **at least one type-resolved call** into **`log/slog`** in its body. Using **`slog.LevelDebug`** is fine for noisy or trivial paths.

**Measure:** `funclogmeasure` (`go run ./cmd/funclogmeasure` or `scripts/measure-func-slog.sh` / `scripts/measure-func-slog.ps1`). It loads **`./...`** with **`go/types`** (via `golang.org/x/tools/go/packages`) and counts any call that resolves to a **`log/slog`** function or method (for example `slog.Info`, `slog.Default().Info`, `logger.Info` when `logger` is a `*slog.Logger`, and dot-imported `Info`).

**Caveats (read before `-enforce`):**

- Calls are matched by **type information**, not text; non-`slog` loggers or `interface{ Info(...) }` calls are **not** counted.
- **Nested function literals** are **not** walked; only the outer named function’s body counts. The outer still needs its own `slog` call if you want it to pass.
- Functions in files with **no** successful type check for their package may be skipped (a warning is logged to stderr).
- Helpers that **only** call non-`slog` wrappers (for example pure `writeStoreError` without a `slog` call inside **this** function) do **not** count.
- **`cmd/funclogmeasure`** is **skipped by default**; use `-include-tool` to audit it too.
- A **tiny allowlist** in `cmd/funclogmeasure` excludes helpers where a per-call `slog` trace would be misleading or hot-path expensive (today: `internal/version.String`; `pkgs/repo.isMentionDelimiter` inside `ParseFileMentions`; `apijson.ApplySecurityHeaders` on every response / scrape; `handler.ServerVersion` as a thin wrapper over `internal/version.String`; `middleware.(*metricsHTTPResponseWriter).{WriteHeader,Write,Flush,statusCode}` on the Prometheus metrics wrapper; black-box test wiring in `internal/handlertest` (`NewServer*`); `internal/httpsecurityexpect.AssertBaselineHeaders`).

We do **not** treat a single percentage as a product SLO. Use the **checklists** below, **`funclogmeasure`** for the per-function log target, and **test coverage** scripts where they still help.

## Signals (today)

| Signal | Role in T2A | Standard |
|--------|----------------|----------|
| **Structured logs** | Primary signal: JSON lines per process run | `slog` with stable keys; errors include `err`; no secrets (see security baseline rule). |
| **Request correlation** | Tie access line, handler errors, and GORM SQL | `request_id` on the request context; response header **`X-Request-ID`** (client echo or server UUID). Health/readiness paths omit the **`http.access`** line but still set the header and context id. |
| **Build identity** | Match JSONL / CLI to binary and health probes | **`internal/version.String()`** (module tag, short **`vcs.revision`**, **`devel`**, or **`unknown`**). `taskapi` logs **`version`** on **`listening`** (`operation` **`taskapi.serve`**); **`dbcheck`** logs **`version`**, **`ping_timeout_sec`** (**`postgres.DefaultPingTimeout`**), and (when **`-migrate`**) **`migrate_timeout_sec`**, on **`dbcheck.start`**, and **`dbcheck.done`** (**`migrate_ran`**) on success; on failure, **`dbcheck failed`** at **Error** with **`operation`** **`dbcheck.failed`** includes **`deadline_exceeded`** when the error chain is **`context.DeadlineExceeded`** (ping vs migrate depending on which step failed). Health JSON uses the same **`version`** via **`handler.ServerVersion()`**. |
| **Startup config** | Confirm DB pool, slow-SQL threshold, HTTP bounds, and middleware knobs (no secrets) | Right after the JSON **`slog`** handler is installed (and not in minimized logging mode), **`taskapi.logging`** records **`min_level`** and **`json_file`** **`true`**; the record’s **`level`** matches **`min_level`** so it is never dropped by the handler floor. After **`taskapi.migrate`** (**`migrate ok`** / **`migrate failed`**, **`timeout_sec`** **120** from **`postgres.DefaultMigrateTimeout`**, **`deadline_exceeded`** on **`migrate failed`** when the bound is hit): **`taskapi.db_config`** (pool caps from **`pkgs/tasks/postgres`**, effective **`gorm_slow_query_ms`**) and **`taskapi.http_limits`** (read/header/idle caps, **`shutdown_timeout_sec`**, **`write_timeout_disabled`** **`true`** — **`http.Server.WriteTimeout`** left unset so **`GET /events`** SSE can stay open; see **`docs/RUNTIME-ENV.md`**). Before the handler stack: **`taskapi.repo_root`** (**`enabled`**, **`path`** when **`REPO_ROOT`** is set and opens cleanly), **`taskapi.rate_limit`** (**`enabled`**, **`per_ip_per_min`** — **`0`** when **`T2A_RATE_LIMIT_PER_MIN`** disables limiting), **`taskapi.max_body`** (**`enabled`**, **`max_bytes`** — **`0`** when **`T2A_MAX_REQUEST_BODY_BYTES`** is unset/disabled), **`taskapi.idempotency`** (**`enabled`**, **`ttl_sec`**). When **`T2A_SSE_TEST`** is not **`1`**, **`taskapi.sse_dev`** logs **`sse dev config`** with **`enabled`** **`false`** (see **SSE dev mode**). Never logs **`DATABASE_URL`**. **`dbcheck`** (stderr text): after **`ping`**, **`dbcheck.db_config`** — same **`postgres.LogStartupDBConfig`** fields as **`taskapi`**, **`operation`** **`cmd`+`.db_config`**. |
| **Graceful shutdown** | Correlate signals with HTTP drain and DB teardown | On SIGINT/SIGTERM: **`shutdown signal received`** (`operation` **`taskapi.shutdown`**, **`signal`**); after successful **`Server.Shutdown`**, **`http server drained`** (`phase` **`http_done`**). After a successful **`sql.DB.Close`**, **`database pool closed`** (`operation` **`taskapi.shutdown`**, **`phase`** **`db_done`**). Before exit code **0**, **`process exit`** with **`phase`** **`exit`**, **`db_closed`**, and **`signal_shutdown`** (**`true`** after SIGINT/SIGTERM-driven drain; **`false`** when **`Serve`** returned **`ErrServerClosed`** or **`nil`** without that path). On failed **`Shutdown`**, the error line includes **`deadline_exceeded`** when the drain hit **`taskapi.http_limits`** **`shutdown_timeout_sec`**. **`database close skipped`** (**`gorm.DB.DB()`** error) or failed pool **`Close`** logs **`taskapi.db_close`** with **`err`** and exits **1** (no **`process exit`** line on that path). |
| **SSE dev mode** | Make synthetic SSE obvious in logs | **`taskapi.sse_dev`**: when **`T2A_SSE_TEST`** is not **`1`**, **`sse dev config`** with **`enabled`** **`false`**. When enabled, either **`sse dev ticker enabled`** with **`interval`**, or **`sse dev env on, ticker off`** when the interval is **`0`** or below **1s** (includes a short **`hint`**). See **`docs/API-SSE.md`**. |
| **Log order** | Sort JSONL within a request or the process | `log_seq` (monotonic) with `log_seq_scope` `request` (access middleware) or `process` (startup, `/health`, background). Implementation: [`pkgs/tasks/logctx`](../pkgs/tasks/logctx/). |
| **Line kind** | Filter JSONL in tools | `obs_category`: `http_access`, `http_io`, `helper_io`. |
| **Access line** | One completion record per HTTP request (except `GET /health`, `/health/live`, `/health/ready`) | `operation` = `http.access`; includes `method`, `path`, `route`, `status`, `duration_ms`, `bytes_written`. |
| **Readiness** | Tell DB probe timeouts from other failures | **`GET /health/ready`**: when the **`database`** check fails, **`readiness check failed`** at **Warn** (**`operation`** **`health.ready`**) includes **`timeout_sec`** (**`store.DefaultReadyTimeout`**, **2**) and **`deadline_exceeded`** when the error chain is **`context.DeadlineExceeded`**. See **`docs/API-HTTP.md`** (health). |
| **Handler panic** | Rare bugs, easier triage | **`operation`** **`http.recover`** at **Error**: **`request_id`**, **`method`**, **`path`**, **`route`** (mux pattern when set), **`duration_ms`** (wall time until panic; there is no `http.access` line when the handler panics), **`panic`**, **`stack`**. Request id is attached in **`WithRecovery`** before inner middleware so correlation matches **`X-Request-ID`**. Client sees **500** JSON (**`internal server error`**). |
| **SQL traces** | DB latency and shape | GORM → same `slog` sink; parameterized SQL; statements slower than **`T2A_GORM_SLOW_QUERY_MS`** (default 200ms, `0` disables) log at **Warn** with elapsed time and SQL in the `trace` group. |
| **Metrics** | Rates, histograms, SLO dashboards | **`GET /metrics`** (Prometheus text): `taskapi_http_*`, `taskapi_sse_subscribers`, and **`taskapi_db_pool_*`** ([`sql.DB.Stats`](https://pkg.go.dev/database/sql#DBStats) on scrape) as in [API-HTTP.md](./API-HTTP.md); plus standard **`go_*`** and **`process_*`** collectors from `taskapi` startup ([OBSERVABILITY-ROADMAP.md](./OBSERVABILITY-ROADMAP.md) phases A2–A3). **`taskapi_http_request_duration_seconds`** uses **SLO-tuned** histogram buckets (finer below 1s; tail to 10s) — see [API-HTTP.md](./API-HTTP.md) Prometheus table and `pkgs/tasks/middleware/metrics_http.go` (`httpRequestDurationSecondsBuckets`). Health paths excluded from HTTP latency series where documented. Per-IP limit: **`T2A_RATE_LIMIT_PER_MIN`**. Idempotency cache TTL: **`T2A_IDEMPOTENCY_TTL`**. Responses include the same baseline security headers as the API (`handler.WrapPrometheusHandler`, headers only—no per-scrape `handler.setAPISecurityHeaders` debug trace). Restrict scrapes in production. |
| **Distributed traces** | Span graphs across services | Not in scope for single-process `taskapi` unless we adopt OpenTelemetry later. |

## Grafana / PromQL (`GET /metrics`)

Scrape **`taskapi`** from Prometheus (or query the text exposition with **`promtool`** in CI). Metric names and labels match [API-HTTP.md](./API-HTTP.md) (Prometheus section).

**Scrape security:** **`GET /metrics`** has **no app-level authentication** — restrict by network, reverse-proxy allowlist, or mTLS in production (see [API-HTTP.md](./API-HTTP.md) and [SECURITY.md](../SECURITY.md)).

Example queries (adjust `[5m]` to your scrape interval and range habits; `job` / `instance` labels depend on your Prometheus `scrape_configs`):

| Goal | PromQL (illustrative) |
|------|------------------------|
| **p95 latency** (all routes) | `histogram_quantile(0.95, sum(rate(taskapi_http_request_duration_seconds_bucket[5m])) by (le))` |
| **p95 latency by `route`** | `histogram_quantile(0.95, sum(rate(taskapi_http_request_duration_seconds_bucket[5m])) by (le, route))` |
| **HTTP 5xx rate** (fraction of requests) | `sum(rate(taskapi_http_requests_total{code=~"5.."}[5m])) / sum(rate(taskapi_http_requests_total[5m]))` |
| **429 rate limit events / s** | `rate(taskapi_http_rate_limited_total[5m])` |
| **SSE subscribers** (instant) | `taskapi_sse_subscribers` |
| **Idempotent replays / s** | `rate(taskapi_http_idempotent_replay_total[5m])` |
| **DB pool waits / s** | `rate(taskapi_db_pool_wait_count_total[5m])` |
| **In-use DB connections** (instant) | `taskapi_db_pool_in_use_connections` |

**Grafana:** add a Prometheus datasource pointing at your scraper, then panels with the expressions above (e.g. time series for p95, stat for 5xx ratio). Use recording rules later if these queries are heavy ([OBSERVABILITY-ROADMAP.md](./OBSERVABILITY-ROADMAP.md) phase B2).

### SLIs and SLOs (roadmap B1)

**Definitions:** an **SLI** is a measurable signal (ratio, quantile, or probe outcome). An **SLO** is a target for that SLI over a **time window** (here: **30 rolling calendar days** unless your org standard differs). The **error budget** is how much “bad” SLI you can spend in that window before the SLO is at risk (e.g. for 99.9% success, budget ≈ **0.1%** bad events).

The three SLIs below are **defaults for `taskapi`** — adjust targets and queries for your traffic, regions, and Postgres tier. Product / SRE owns the final numbers.

| # | SLI | What we measure | Example expression | Starting SLO target |
|---|-----|------------------|--------------------|---------------------|
| **1** | **HTTP success** (no server `5xx`) | Share of responses that are not `5xx`, from `taskapi_http_requests_total`. | Good ratio over a range: `1 - (sum(rate(taskapi_http_requests_total{code=~"5.."}[5m])) / sum(rate(taskapi_http_requests_total[5m])))` | **99.9%** of requests not `5xx` over **30d** (budget ≈ **0.1%** `5xx`; excludes clients you do not count—define explicitly). |
| **2** | **Mutating API latency** | **p99** request duration for `POST` / `PATCH` / `DELETE` (task and related routes share the same histogram labels). | `histogram_quantile(0.99, sum(rate(taskapi_http_request_duration_seconds_bucket{method=~"POST|PATCH|DELETE"}[5m])) by (le))` | **p99 < 2s** over **30d** (tighten when store and DB are sized; see slow-query logs for grounding). |
| **3** | **Dependency readiness** | **`GET /health/ready`** returns **200** with `checks.database` (and `workspace_repo` when configured) — usually measured with a **blackbox** or synthetic probe, not from `taskapi_http_*` alone (health is omitted from that histogram). | Probe success rate from your checker (e.g. Prometheus **blackbox_exporter** HTTP prober, or k8s readiness success). | **99.5%** successful ready checks over **30d** (raise when DB or disk is critical path). |

**Alternative third SLI (Prometheus-native):** if you do not yet have blackbox metrics, use **DB pool health**: e.g. alert when `rate(taskapi_db_pool_wait_count_total[5m])` is sustained above a small threshold (tune per pool size); document the threshold as the SLO.

**Error budget in practice:** roll a **30d** window in Grafana or Mimir; use **multi-window, multi-burn-rate** alerts so short spikes do not page while slow burns do ([OBSERVABILITY-ROADMAP.md](./OBSERVABILITY-ROADMAP.md) phase **B2**). Example: for SLI 1 at 99.9%, a **5m** burn at 10× normal `5xx` rate consumes budget quickly — encode in alert rules when you add them.

### 5xx and `request failed` logging (roadmap A5)

- **`http.access`** (`operation` **`http.access`**, non-probe routes): includes **`duration_ms`**, **`status`**, **`route`**, **`path`**, **`method`**; **`request_id`** is added by the JSON `slog` handler from context when present.
- **`request failed`** (`writeError` / `writeStoreError`, **`logRequestFailure`**): **`operation`**, **`http_status`**, **`err`**, explicit **`request_id`** and **`route`** (for grep and Loki-style queries). **`duration_ms`** for the same request appears on the companion **`http.access`** line when the request completes without panicking.
- **Panics** (`middleware.WithRecovery`): **`http.recover`** line includes **`request_id`**, **`route`**, **`duration_ms`** because the access middleware does not run to completion on panic.
- **JSON encode failures** (`response encode failed` in **`handler`** / **`apijson`**): **`request_id`**, **`route`** (and **`method`**/**`path`** in **`apijson`**) where the request is available.
- **Idempotency singleflight errors**: **`middleware.idempotency`** log includes **`request_id`**, **`method`**, **`path`**, **`route`**.

## Checklist: increasing observability

When you add or materially change behavior, use this list (copy into a PR description if helpful).

### HTTP handlers (`pkgs/tasks/handler`)

- [ ] **Context:** Handlers use `r.Context()` for store calls so SQL logs share `request_id`.
- [ ] **Failures:** Use `writeError` / `writeStoreError` (or `slog.Log(r.Context(), …)`) so client/server errors keep correlation.
- [ ] **SSE:** Long streams still get one access line at the end; publish path uses `slog` appropriately (see `tasks.sse.publish` at Debug when subscribers exist).
- [ ] **Operations:** New code paths use a stable, grep-friendly `operation` string (existing pattern: `tasks.*`, `repo.*`, `http.*`).
- [ ] **IO visibility:** At Debug, `http.io` lines record `phase` `in`/`out`, handler `operation`, `call_path`, and safe input/output summaries; helpers emit `helper.io` with `function` and the same `call_path` (see `pkgs/tasks/calltrace`, `docs/API-HTTP.md` — structured logs). Use `calltrace.RunObserved` when a helper should log explicit input/output key/value pairs. New routes: `calltrace.WithRequestRoot(r, op)` first; pass `r.Context()` into helpers that support `calltrace.Push`—avoid secrets and unbounded payloads.

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

**CI and `scripts/check`:** after a successful `go test ./...`, `./scripts/check.sh` and `.\scripts\check.ps1` run `go run ./cmd/funclogmeasure -enforce` unless **`CHECK_SKIP_FUNCLOG=1`** is set (local escape hatch only; GitHub Actions always enforces).

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

When we introduce either, update this doc and `docs/API-HTTP.md` (`GET /metrics`) so operators know how to scrape or export data.

## Related docs and rules

- [API-HTTP.md](./API-HTTP.md) — logging, `X-Request-ID`, access line, GORM SQL (handler section).
- [RUNTIME-ENV.md](./RUNTIME-ENV.md) — log env vars and startup.
- [AGENTS.md](../AGENTS.md) — commands including measurement script.
- `.cursor/rules/04-structured-logging.mdc` — field names, levels, secrets.
- `.cursor/rules/09-security-baseline.mdc` — no credentials in logs.
