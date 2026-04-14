# `pkgs/tasks/middleware`

Standard **outer** HTTP middleware for `taskapi`: recovery, metrics, access log, rate limit, optional auth, timeouts, body cap, idempotency.

**Dependencies:** only [`pkgs/tasks/apijson`](../apijson/) and [`pkgs/tasks/logctx`](../logctx/) — **no** import of [`pkgs/tasks/handler`](../handler/) (avoids cycles). Production wiring is **`internal/taskapi.NewHTTPHandler`** → `Stack(handler.NewHandler(...), calltrace.Path)`.

**Contracts and env overview:** [docs/DESIGN.md](../../docs/DESIGN.md), [docs/RUNTIME-ENV.md](../../docs/RUNTIME-ENV.md). REST/SSE behavior lives in [`pkgs/tasks/handler/README.md`](../handler/README.md).

## `Stack` order (outer → inner)

Defined in **`stack.go`**. When changing order or adding a layer, update this README, [`stack.go`](stack.go), and [docs/OBSERVABILITY.md](../../docs/OBSERVABILITY.md) if logs or metrics change.

| Layer | Role |
|-------|------|
| `WithRecovery` | Panic → 500 JSON. |
| `WithHTTPMetrics` | Prometheus `taskapi_http_*`; health paths omitted from latency where documented in code. |
| `WithAccessLog` | Structured `http.access` line; `call_path` from the injected `callPath` func (use `calltrace.Path` in production). |
| `WithRateLimit` | Per-IP limit; see `T2A_RATE_LIMIT_PER_MIN` below. |
| `WithAPIAuth` | Optional `Authorization: Bearer`; see `T2A_API_TOKEN`. |
| `WithRequestTimeout` | Request context deadline; `GET /events` exempt. |
| `WithMaxRequestBody` | Max body bytes before handler runs. |
| `WithIdempotency` | Mutating requests with `Idempotency-Key`; TTL and cache caps from env. |

## Source files

| File | Role |
|------|------|
| `stack.go` | `Stack(inner, callPath)` composes the chain. |
| `recovery.go` | Panic handler. |
| `metrics_http.go` | HTTP metrics + SSE subscriber gauge (`RecordSSESubscriberGauge`). HTTP latency histogram uses **SLO-tuned** buckets (`httpRequestDurationSecondsBuckets`, not `prometheus.DefBuckets`). |
| `accesslog.go` | Access logging. |
| `rate_limit.go` | Token-bucket rate limit. |
| `api_auth.go` | Bearer token gate. |
| `request_timeout.go` | `T2A_HTTP_REQUEST_TIMEOUT`. |
| `max_body.go` | `T2A_MAX_REQUEST_BODY_BYTES`. |
| `idempotency.go` | Idempotency middleware + `IdempotencyTTL` / `IdempotencyCacheLimits`. |
| `idempotency_cache.go` | In-process replay cache (`ClearIdempotencyStateForTest` for tests). |

## Environment variables (read in this package)

| Variable | Used by | Notes |
|----------|---------|--------|
| `T2A_RATE_LIMIT_PER_MIN` | `WithRateLimit` | Default 120/min; `0` disables. |
| `T2A_API_TOKEN` | `WithAPIAuth` | Non-empty enables bearer auth on API routes. |
| `T2A_HTTP_REQUEST_TIMEOUT` | `WithRequestTimeout` | Go duration; default 30s; `0` disables. |
| `T2A_MAX_REQUEST_BODY_BYTES` | `WithMaxRequestBody` | Default 1 MiB; `0` unlimited. |
| `T2A_IDEMPOTENCY_TTL` | `WithIdempotency` | Default 24h; `0` disables caching. |
| `T2A_IDEMPOTENCY_MAX_ENTRIES` | idempotency cache | Default 2048; `0` disables entry cap. |
| `T2A_IDEMPOTENCY_MAX_BYTES` | idempotency cache | Default 8 MiB; `0` disables byte cap. |

Taskapi-only knobs (listen address, log level, agent intervals, etc.) are **not** here — see [`internal/taskapiconfig`](../../internal/taskapiconfig/).

## Tests

| Location | What belongs there |
|----------|-------------------|
| This directory (`package middleware`) | Whitebox tests that need unexported symbols (e.g. rate-limit IP parsing, idempotency cache internals, Prometheus vec handles). |
| [`internal/middlewaretest`](../../internal/middlewaretest/) (`package middlewaretest`) | Black-box tests that only use the exported `middleware` API (recovery, request timeout, max-body env parsing). |
| [`pkgs/tasks/handler`](../handler/) | Integration tests that compose `With*` through [`middleware_shim.go`](../handler/middleware_shim.go). |

`go test ./...` from the repo root runs both trees.

When adding a **new** middleware file, extend this README in the same PR.
