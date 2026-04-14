# Runbook: TaskAPIHighMutatingLatencyP99

## What it means

**p99** latency for **`POST` / `PATCH` / `DELETE`** exceeded **5 seconds** for **15 minutes** (recording rule `taskapi:http:mutating_p99_seconds`). Mutating traffic shares one histogram with other API routes; a spike here often reflects **database** or **payload** work, not scrape noise.

## Severity and escalation

- **Default:** `severity: warning` — treat as **performance SLO risk** (SLI 2 in [OBSERVABILITY.md](../OBSERVABILITY.md)).
- **Escalate** if p99 stays high while **`taskapi_db_pool_wait_count_total`** climbs: involve **database on-call** before only scaling app replicas.

## Dashboards (Prometheus / Grafana)

1. **Alert signal:**

   ```promql
   taskapi:http:mutating_p99_seconds
   ```

2. **Which routes drive tail latency:**

   ```promql
   histogram_quantile(
     0.99,
     sum by (le, route) (
       rate(taskapi_http_request_duration_seconds_bucket{method=~"POST|PATCH|DELETE"}[5m])
     )
   )
   ```

3. **Pool pressure:**

   ```promql
   rate(taskapi_db_pool_wait_count_total[5m])
   ```

   ```promql
   taskapi_db_pool_in_use_connections
   ```

4. **SSE load** (long-lived connections reduce available capacity on small hosts):

   ```promql
   taskapi_sse_subscribers
   ```

## Logs (JSONL)

**GORM slow queries** (when `T2A_GORM_SLOW_QUERY_MS` is set): search for the slow-query **Warn** pattern your deployment documents; correlate **`request_id`** with **`http.access`** **`route`** for the same request.

**Access lines** for mutating methods (high **`duration_ms`**):

```bash
rg '"operation":"http.access".*"method":"(POST|PATCH|DELETE)"' /var/log/taskapi/*.log | head
```

Narrow by **`route`** once you know the hot path from PromQL.

## Check first

1. **Histogram:** `taskapi_http_request_duration_seconds_bucket` by `route` and `method` (queries above).
2. **DB:** `taskapi_db_pool_*` gauges and wait rate; Postgres active sessions and locks.
3. **In-flight overload:** `taskapi_http_in_flight` sustained high (see [alert-in-flight-high.md](./alert-in-flight-high.md)).

## Mitigations

- Increase **Postgres** capacity or **pool** limits only after validating the database can take more connections; prefer **indexes**, **smaller payloads**, or **removing N+1** query patterns.
- **Shed load** at the proxy during incidents; fix **retry storms** from clients (backoff, jitter).
