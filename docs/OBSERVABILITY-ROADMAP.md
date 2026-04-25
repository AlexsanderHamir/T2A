# Observability roadmap (T2A)

Industry-aligned plan for **logs + Prometheus + (later) tracing**: RED-style HTTP metrics, USE-style resource signals, bounded cardinality, and operator-friendly docs. Parent standard: [OBSERVABILITY.md](./OBSERVABILITY.md).

## Open todos

Use this list in order unless a later item unblocks an incident. Earlier items (A1–C3 — runtime / DB-pool / histogram-bucket / log-audit metrics, SLIs+SLOs, alerting, runbooks, domain + store-latency + build-info metrics) have shipped; their contracts live in [OBSERVABILITY.md](./OBSERVABILITY.md), [API-HTTP.md](./API-HTTP.md), and [`docs/runbooks/`](./runbooks/).

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
