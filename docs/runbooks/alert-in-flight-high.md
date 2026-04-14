# Runbook: TaskAPIHTTPInFlightHigh

## What it means

**`taskapi_http_in_flight`** stayed above **200** for **10m** (tune for your max concurrency).

## Check first

1. **p95/p99** latency and **5xx** rate (cascading slowdown).
2. **SSE:** `taskapi_sse_subscribers` and long-lived **`GET /events`**.
3. **Downstream:** Postgres connections, CPU on the `taskapi` host.

## Mitigations

- Add replicas or scale vertically; fix slow queries; cap abusive clients (rate limit already per IP — verify **`T2A_RATE_LIMIT_PER_MIN`**).
