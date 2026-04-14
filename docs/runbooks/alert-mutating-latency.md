# Runbook: TaskAPIHighMutatingLatencyP99

## What it means

**p99** latency for **`POST` / `PATCH` / `DELETE`** exceeded **5s** for **15m** (recording rule `taskapi:http:mutating_p99_seconds`).

## Check first

1. **Histogram:** `taskapi_http_request_duration_seconds_bucket` by `route` and `method`.
2. **DB:** GORM slow-query **Warn** logs (`T2A_GORM_SLOW_QUERY_MS`); **`taskapi_db_pool_*`** gauges and wait rate.
3. **In-flight:** `taskapi_http_in_flight` for overload.

## Mitigations

- Increase DB pool / instance class; add indexes or fix N+1 queries; reduce payload size or batch work; temporarily shed load at the proxy.
