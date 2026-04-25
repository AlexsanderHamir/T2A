# Runbooks (T2A)

Operator playbooks for teams that wire alerts around `taskapi` runtime signals. Each runbook includes **PromQL**, **log** correlation ideas (`rg` on JSONL; adapt paths and **`jq`** if you ship logs to Loki/Elastic), and **escalation** hints.

**Prerequisites:** scrape **`GET /metrics`** securely ([API-HTTP.md](../API-HTTP.md)); know your log path and JSON field names ([OBSERVABILITY.md](../OBSERVABILITY.md)). Use **`taskapi_build_info`** to confirm which **version / revision** is running during rollouts.

| Alert / topic | Runbook |
|---------------|---------|
| `TaskAPIHighHTTP5xxRate` | [alert-http-5xx.md](./alert-http-5xx.md) |
| `TaskAPIHighMutatingLatencyP99` | [alert-mutating-latency.md](./alert-mutating-latency.md) |
| `TaskAPIHTTPInFlightHigh` | [alert-in-flight-high.md](./alert-in-flight-high.md) |
| `TaskAPIDatabasePoolWaitElevated` | [alert-db-pool-wait.md](./alert-db-pool-wait.md) |
| `TaskAPISSEDroppedFramesElevated` | [alert-sse-dropped-frames.md](./alert-sse-dropped-frames.md) |
| Readiness (external probe) | [alert-readiness.md](./alert-readiness.md) |

**Roadmap:** further metrics (domain counters, store latency) are listed in [OBSERVABILITY-ROADMAP.md](../OBSERVABILITY-ROADMAP.md).
