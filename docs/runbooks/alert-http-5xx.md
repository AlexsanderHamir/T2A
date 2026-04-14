# Runbook: TaskAPIHighHTTP5xxRate

## What it means

Server-side **`5xx`** responses are a significant share of HTTP traffic for at least **15 minutes** (default threshold **2%** while RPS > **0.1**).

## Check first

1. **Grafana / Prometheus:** `taskapi:http:5xx_ratio5m` and `sum by (route, code) (rate(taskapi_http_requests_total[5m]))`.
2. **Logs (JSON):** lines with **`operation`** `http.recover` (panic) or **`request failed`** at **Error**; correlate **`request_id`** with **`http.access`** for the same request.
3. **`GET /health/ready`:** database or workspace repo degraded ([API-HTTP.md](../API-HTTP.md) health).

## Mitigations

- Roll back recent deploy; scale Postgres / `taskapi` replicas; drain bad traffic at the gateway.
- If a single **`route`** dominates, isolate that handler (see `pkgs/tasks/handler`).
