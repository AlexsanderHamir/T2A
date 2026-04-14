# Runbook: TaskAPIDatabasePoolWaitElevated

## What it means

**`rate(taskapi_db_pool_wait_count_total[5m])`** exceeded **5/s** for **10m** — goroutines are often blocked waiting for a **`database/sql`** connection.

## Check first

1. **Gauges:** `taskapi_db_pool_in_use_connections`, `taskapi_db_pool_open_connections`, `taskapi_db_pool_max_open_connections`.
2. **Postgres:** active sessions, locks, disk I/O.
3. **App:** slow queries and handler paths holding connections too long.

## Mitigations

- Raise **`SetMaxOpenConns`** only after validating Postgres can serve more connections; prefer fixing slow work or scaling the database.
