# Runbooks (T2A)

Short operator notes for alerts shipped in [`deploy/prometheus/t2a-taskapi-rules.yaml`](../../deploy/prometheus/t2a-taskapi-rules.yaml). Expand per alert in [OBSERVABILITY-ROADMAP.md](../OBSERVABILITY-ROADMAP.md) phase **B3**.

| Alert | Runbook |
|-------|---------|
| `TaskAPIHighHTTP5xxRate` | [alert-http-5xx.md](./alert-http-5xx.md) |
| `TaskAPIHighMutatingLatencyP99` | [alert-mutating-latency.md](./alert-mutating-latency.md) |
| `TaskAPIHTTPInFlightHigh` | [alert-in-flight-high.md](./alert-in-flight-high.md) |
| `TaskAPIDatabasePoolWaitElevated` | [alert-db-pool-wait.md](./alert-db-pool-wait.md) |
| Readiness (external probe) | [alert-readiness.md](./alert-readiness.md) |
