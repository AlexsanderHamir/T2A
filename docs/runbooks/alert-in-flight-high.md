# Runbook: TaskAPIHTTPInFlightHigh

## What it means

**`taskapi_http_in_flight`** stayed above **200** for **10 minutes**. This is a **capacity / stall** signal: many requests are accepted but not finishing. Threshold is a starting point — tune for your expected concurrency and instance size.

## Severity and escalation

- **Default:** `severity: warning`.
- **Escalate** if in-flight stays high **with** rising **`5xx`** or **readiness** failures: treat as **outage** coordination (on-call + DB + edge).

## Dashboards (Prometheus / Grafana)

1. **In-flight (instant):**

   ```promql
   taskapi_http_in_flight
   ```

2. **Same window: error rate and latency** (cascading slowdown):

   ```promql
   taskapi:http:5xx_ratio5m
   ```

   ```promql
   taskapi:http:p95_seconds
   ```

3. **SSE clients** (each holds a connection):

   ```promql
   taskapi_sse_subscribers
   ```

4. **DB pool** (handlers blocked on `sql.DB`):

   ```promql
   taskapi_db_pool_in_use_connections
   ```

   ```promql
   rate(taskapi_db_pool_wait_count_total[5m])
   ```

## Logs (JSONL)

High in-flight often pairs with **`http.access`** lines showing large **`duration_ms`** or **`request failed`** at **Error**.

**Sample slow completions** (adjust path):

```bash
rg '"operation":"http.access"' /var/log/taskapi/*.log | rg '"duration_ms":([5-9][0-9]{3}|[0-9]{5,})'
```

**429 storms** (clients hammering retries):

```bash
rg '"operation":"http.rate_limit"' /var/log/taskapi/*.log
```

## Check first

1. **p95 / p99** latency and **5xx** ratio (recording rules above).
2. **`GET /events`:** `taskapi_sse_subscribers` and proxy idle timeouts.
3. **Host:** CPU, file descriptors, and Postgres session count.

## Mitigations

- Add **`taskapi`** replicas or scale vertically; fix **slow queries** and **long transactions**.
- **`T2A_RATE_LIMIT_PER_MIN`** is per-instance, not global — use **gateway** limits for coordinated abuse.
- If **SSE** dominates, consider dedicated nodes or higher **`http.Server`** limits only after validating the OS and proxy can sustain them.
