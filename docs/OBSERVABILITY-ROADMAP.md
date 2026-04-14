# Observability roadmap (T2A)

Industry-aligned plan for **logs + Prometheus + (later) tracing**: RED-style HTTP metrics, USE-style resource signals, bounded cardinality, and operator-friendly docs. Parent standard: [OBSERVABILITY.md](./OBSERVABILITY.md).

## Execution todos

Use this list in order unless a later item unblocks an incident.

- [x] **A2 — Runtime / process metrics:** Register Prometheus `GoCollector` and `ProcessCollector` on the default registry so `GET /metrics` exposes GC, goroutines, memory, and process RSS/CPU (see implementation in `internal/taskapi` + `cmd/taskapi/run.go`).
- [x] **A3 — DB pool metrics:** Custom Prometheus collector reads [`sql.DB.Stats`](https://pkg.go.dev/database/sql#DBStats) on each scrape (`taskapi_db_pool_*` on `GET /metrics`); wired from `cmd/taskapi` via `taskapi.RegisterSQLDBPoolCollector` ([`internal/taskapi/db_pool_collector.go`](../internal/taskapi/db_pool_collector.go)).
- [x] **A4 — Histogram buckets:** `taskapi_http_request_duration_seconds` uses `httpRequestDurationSecondsBuckets` in [`pkgs/tasks/middleware/metrics_http.go`](../pkgs/tasks/middleware/metrics_http.go) (denser ≤1s, tail to 10s); documented in [API-HTTP.md](./API-HTTP.md) and [OBSERVABILITY.md](./OBSERVABILITY.md).
- [x] **A1 — Operator PromQL:** [OBSERVABILITY.md](./OBSERVABILITY.md) § **Grafana / PromQL** — example queries (p95 overall and by `route`, 5xx ratio, rate limit / idempotency rates, SSE gauge, DB pool); `/metrics` scrape authz called out.
- [x] **A5 — Log audit:** `WithRecovery` assigns **`request_id`** before inner handlers; panic logs include **`request_id`**, **`route`**, **`duration_ms`**; **`logRequestFailure`** / JSON encode / idempotency **`5xx`** paths log **`request_id`** + **`route`**; **`http.access`** supplies **`duration_ms`** for completed requests ([OBSERVABILITY.md](./OBSERVABILITY.md) § **5xx and `request failed` logging**).
- [x] **B1 — SLIs / SLOs:** [OBSERVABILITY.md](./OBSERVABILITY.md) § **SLIs and SLOs** — three starter SLIs (HTTP success vs `5xx`, mutating p99 latency, readiness / optional DB pool), **30d** window, error budget framing; targets are defaults to tune.
- [ ] **B2 — Alerting:** Prometheus `rules.yml` (recording + alerts): high 5xx, p99 regression, in-flight sustained high, pool wait spikes, readiness failures; link runbooks.
- [ ] **B3 — Runbooks:** One page per alert: graphs, log queries, mitigations.
- [ ] **C1 — Domain metrics:** Low-cardinality counters (tasks created/updated, agent queue depth, idempotency evictions) as needed from real incidents.
- [ ] **C2 — Store latency:** Optional labeled histogram for store ops (`op` from a small fixed set), not per-SQL-string.
- [ ] **C3 — Build info:** `taskapi_build_info{version="...",revision="..."} = 1` gauge for deploy correlation on dashboards.
- [ ] **D1 — OpenTelemetry:** Traces for `taskapi` + OTLP export when multi-service or deep latency debugging is required.
- [ ] **D2 — Exemplars / log correlation:** Trace IDs on spans and in `slog`; histogram exemplars where backend supports it.

## Principles (do not regress)

- **Low cardinality:** No per-user or per-task-id labels on high-frequency series; keep `route` as mux pattern.
- **Secure metrics:** `/metrics` stays unauthenticated at the app — restrict at the network or gateway (see [API-HTTP.md](./API-HTTP.md)).
- **Health traffic:** Keep health probes out of HTTP SLI histograms where already excluded (see `middleware.omitHTTPMetrics`).

## Related docs

- [OBSERVABILITY.md](./OBSERVABILITY.md) — current signals, `funclogmeasure`, checklists.
- [API-HTTP.md](./API-HTTP.md) — `GET /metrics` contract and metric names.
- [DESIGN.md](./DESIGN.md) — hub; tracing called out as future.
