# `pkgs/tasks/handler`

HTTP surface for `taskapi`: REST + optional `/repo` + `GET /events` (SSE). **Contracts:** [docs/API-HTTP.md](../../docs/API-HTTP.md), [docs/API-SSE.md](../../docs/API-SSE.md). **How to extend:** [docs/EXTENSIBILITY.md](../../docs/EXTENSIBILITY.md).

The returned `http.Handler` from `NewHandler` is the **inner mux** (routes only). `cmd/taskapi` mounts it behind the standard stack from **`handler.MiddlewareStack`**, invoked by **`internal/taskapi.NewHTTPHandler`**. Wiring order and devsim live in **`cmd/taskapi/run.go`**. Taskapi-only env parsing lives in **`internal/taskapiconfig`**.

## Middleware (`With*` — outer stack from `MiddlewareStack`)

| Middleware | File | Role |
|------------|------|------|
| `WithRecovery` | `recovery.go` | Panic → 500 JSON. |
| `WithHTTPMetrics` | `metrics_http.go` | Prometheus `taskapi_http_*` + in-flight gauge (health paths excluded from latency). |
| `WithAccessLog` | `accesslog.go` | `http.access` line, `request_id`, `log_seq` scope. |
| `WithRateLimit` | `rate_limit.go` | Per-IP token bucket (`T2A_RATE_LIMIT_PER_MIN`). |
| `WithAPIAuth` | `api_auth.go` | Optional bearer token (`T2A_API_TOKEN`). |
| `WithRequestTimeout` | `request_timeout.go` | Context deadline on API routes; SSE exempt. |
| `WithMaxRequestBody` | `max_body.go` | Body size cap (`T2A_MAX_REQUEST_BODY_BYTES`). |
| `WithIdempotency` | `idempotency.go`, `idempotency_cache.go` | `Idempotency-Key` replay cache. |

`stack.go` defines **`MiddlewareStack`**, which composes the `With*` layers in the order above.

`GET /metrics` is registered on the **outer** mux in `cmd/taskapi` (not on the inner handler mux).

## Core mux and types

| File | Role |
|------|------|
| `handler.go` | `Handler`, `NewHandler`, route registration, `notifyChange` / SSE publish wiring, JSON security header helpers. |
| `sse.go` | `SSEHub`, `streamEvents` (`GET /events`). |

## Route handlers (inner mux)

| Area | Files |
|------|--------|
| Health | `handler_health.go` |
| Tasks CRUD + list + stats | `handler_task_crud.go` |
| Checklist | `handler_checklist.go` |
| Task audit / events | `handler_task_events.go` |
| Draft evaluation (`POST /tasks/evaluate`) | `handler_task_evaluation.go` |
| Saved task drafts (`/task-drafts`) | `handler_task_drafts.go` |
| Workspace `/repo/*` | `repo_handlers.go` |

## Request/response helpers

| File | Role |
|------|------|
| `handler_http_json.go` | `decodeJSON`, `writeJSON` / `writeError`, `actorFromRequest`, store error → HTTP. |
| `handler_task_json.go` | Request/response DTOs (`taskCreateJSON`, tree encoding, etc.). |
| `handler_path_ids.go` | Path UUID / segment parsing and abuse-guard caps. |
| `patch_fields.go` | `PATCH` helpers (e.g. nullable `parent_id`). |
| `server_version.go` | Build/version string for health JSON. |

## Observability and debug logging

| File | Role |
|------|------|
| `calllog.go` | `withCallRoot`, `PushCall`, `call_path` for nested handler/helper traces. |
| `observe.go` | `RunObserved` for structured helper in/out pairs. |
| `httplog_io.go` | `http.io` / `helper.io` debug summaries. |
| `log_seq.go`, `requestctx.go`, `slog_requestctx.go` | Per-request log sequence and context wiring. |

## Tests

`handler_http*.go`, `*_test.go` beside the feature under test (`handler_http_checklist_test.go`, `idempotency_test.go`, `sse_test.go`, etc.). **`stack_test.go`** asserts the production **`MiddlewareStack`** (panic → JSON 500, happy path). Integration-style tests may use `handler_http_testserver_test.go` helpers.

When adding a **new** route or middleware file, extend this README in the same PR.
