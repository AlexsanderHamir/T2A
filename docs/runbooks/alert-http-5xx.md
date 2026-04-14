# Runbook: TaskAPIHighHTTP5xxRate

## What it means

Server-side **`5xx`** responses are a significant share of HTTP traffic for at least **15 minutes** (default threshold **2%** while RPS > **0.1**). This alert uses `taskapi_http_requests_total` — the same family as SLI 1 in [OBSERVABILITY.md](../OBSERVABILITY.md).

## Severity and escalation

- **Default label:** `severity: warning` — page on-call for the service that owns `taskapi` if your routing treats warning as page-worthy; otherwise acknowledge and work within business hours unless paired with customer impact.
- **Escalate** when `5xx` correlates with deploy time, Postgres outage, or coordinated client retries: involve **database on-call** and whoever controls the **edge / gateway** (rate limits, WAF).

## Dashboards (Prometheus / Grafana)

1. **Recording rule ratio (matches alert intent):**

   ```promql
   taskapi:http:5xx_ratio5m
   ```

2. **Which routes and codes burn budget:**

   ```promql
   sum by (route, code) (rate(taskapi_http_requests_total[5m]))
   ```

3. **Confirm binary version on each instance** (rollout regression vs infra-wide):

   ```promql
   max by (instance, version, revision) (taskapi_build_info)
   ```

4. **Global p95** (slow handlers often precede timeouts that surface as 5xx):

   ```promql
   taskapi:http:p95_seconds
   ```

## Logs (JSONL)

Structured logs use **`operation`** and **`request_id`** (see [OBSERVABILITY.md](../OBSERVABILITY.md)). Prefer **`X-Request-ID`** from a failing client response when grepping.

**Panics** (rare bugs):

```bash
rg '"operation":"http.recover"' /var/log/taskapi/*.log
```

**Handler / store failures** (return path before access line may still log):

```bash
rg '"msg":"request failed"' /var/log/taskapi/*.log
```

**Correlate one request** (replace `REQUEST_ID`):

```bash
rg 'REQUEST_ID' /var/log/taskapi/*.log
```

You should see **`http.access`** with **`status`**, **`route`**, **`duration_ms`** when the request completed without panic; panics omit the normal access completion line but **`http.recover`** includes **`request_id`** and **`route`**.

## Check first

1. **Recent deploy:** compare `taskapi_build_info` **`revision`** across instances to git history.
2. **`GET /health/ready`:** `503` with `checks.database` or `workspace_repo` — see [API-HTTP.md](../API-HTTP.md) health.
3. **Postgres:** connection limits, disk full, long transactions (outside this repo’s scope but the usual root cause).

## Mitigations

- **Roll back** a bad deploy; **scale** Postgres or `taskapi` replicas; **drain** abusive traffic at the gateway.
- If a single **`route`** dominates in `taskapi_http_requests_total`, isolate that handler (`pkgs/tasks/handler`) and add a targeted fix or temporary feature flag if you have one.
