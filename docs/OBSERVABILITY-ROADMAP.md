# Observability roadmap (T2A)

Industry-aligned plan for **logs + Prometheus + (later) tracing**: RED-style HTTP metrics, USE-style resource signals, bounded cardinality, and operator-friendly docs. Parent standard: [OBSERVABILITY.md](./OBSERVABILITY.md).

## Execution todos

Use this list in order unless a later item unblocks an incident.

- [x] **A2 — Runtime / process metrics:** Register Prometheus `GoCollector` and `ProcessCollector` on the default registry so `GET /metrics` exposes GC, goroutines, memory, and process RSS/CPU (see implementation in `internal/taskapi` + `cmd/taskapi/run.go`).
- [x] **A3 — DB pool metrics:** Custom Prometheus collector reads [`sql.DB.Stats`](https://pkg.go.dev/database/sql#DBStats) on each scrape (`taskapi_db_pool_*` on `GET /metrics`); wired from `cmd/taskapi` via `taskapi.RegisterSQLDBPoolCollector` ([`internal/taskapi/db_pool_collector.go`](../internal/taskapi/db_pool_collector.go)).
- [ ] **A4 — Histogram buckets:** Replace default HTTP latency buckets with SLO-tuned buckets; document in [API-HTTP.md](./API-HTTP.md) and [OBSERVABILITY.md](./OBSERVABILITY.md).
- [ ] **A1 — Operator PromQL:** Add a short “Grafana / PromQL” section to [OBSERVABILITY.md](./OBSERVABILITY.md) — p95 by `route`, 5xx rate, SSE gauge, rate-limit counter rate; note `/metrics` scrape authz.
- [ ] **A5 — Log audit:** Checklist pass so every `5xx` path logs `request_id`, `operation`, `route`, `duration_ms` where applicable.
- [ ] **B1 — SLIs / SLOs:** Pick 2–3 SLIs (e.g. mutating task routes p99, global 5xx rate, readiness); define 30d windows and error budgets.
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
