# Runbook: TaskAPIDatabasePoolWaitElevated

## What it means

**`rate(taskapi_db_pool_wait_count_total[5m])`** exceeded **5/s** for **10 minutes**. Goroutines are frequently **blocked waiting for a free** `database/sql` connection from the pool. This is a strong signal of **pool exhaustion** or **slow queries holding connections**.

## Severity and escalation

- **Default:** `severity: warning`.
- **Escalate** when wait rate rises **and** application **`5xx`** or **timeouts** increase: page **database on-call** — the fix may be on the database side (locks, disk, max connections).

## Dashboards (Prometheus / Grafana)

1. **Alert signal:**

   ```promql
   rate(taskapi_db_pool_wait_count_total[5m])
   ```

2. **Pool saturation:**

   ```promql
   taskapi_db_pool_in_use_connections
   ```

   ```promql
   taskapi_db_pool_open_connections
   ```

   ```promql
   taskapi_db_pool_max_open_connections
   ```

3. **Cumulative wait time** (seconds blocked per second of wall time):

   ```promql
   rate(taskapi_db_pool_wait_duration_seconds_total[5m])
   ```

4. **HTTP tail latency** (mutating p99 often moves with pool wait):

   ```promql
   taskapi:http:mutating_p99_seconds
   ```

## Logs (JSONL)

Correlate **slow SQL** and **handler** work using **`request_id`** ([OBSERVABILITY.md](../OBSERVABILITY.md)).

**Readiness timeouts** when the DB is overloaded:

```bash
rg '"operation":"health.ready"' /var/log/taskapi/*.log
```

**Access lines** with very high **`duration_ms`** on mutating routes often mean the request held a DB connection for most of that time.

## Check first

1. **Gauges:** `taskapi_db_pool_in_use_connections` vs `taskapi_db_pool_max_open_connections`.
2. **Postgres:** `pg_stat_activity`, blocking queries, replication lag, disk I/O.
3. **App:** N+1 patterns, missing indexes, or handlers doing non-DB work while a transaction stays open (anti-pattern).

## Mitigations

- **Do not** blindly raise `SetMaxOpenConns` unless Postgres **`max_connections`** and instance class can absorb it; prefer **faster queries**, **smaller transactions**, and **scaling the database**.
- **Scale `taskapi`** only helps if the bottleneck is CPU on the app host, not Postgres capacity.
- After mitigation, verify **`rate(taskapi_db_pool_wait_count_total[5m])`** returns to baseline before closing the incident.
