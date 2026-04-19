# Runbook: Readiness / `GET /health/ready`

## Note

**`taskapi` HTTP metrics** intentionally **omit** health probe paths from the latency histogram and typical access-log volume ([API-HTTP.md](../API-HTTP.md)). **Readiness** is therefore usually monitored with a **synthetic probe** (Prometheus **blackbox_exporter**, Kubernetes **`readinessProbe`**, or load balancer health checks), not only from `taskapi_http_*`.

## Example alert (Prometheus)

Uncomment and adapt **`TaskAPIReadinessProbeFailing`** in [`deploy/prometheus/t2a-taskapi-rules.yaml`](../../deploy/prometheus/t2a-taskapi-rules.yaml) once you have a metric such as **`probe_success{job="blackbox-taskapi-ready"}`** from blackbox scraping **`GET /health/ready`**.

## Severity and escalation

- Treat repeated **`503`** from **`/health/ready`** as **stop routing traffic** to that instance (orchestrator will do this when the probe fails).
- **Escalate** to **database on-call** when `checks.database` fails; to **platform / config** when `workspace_repo` fails after a volume or **`app_settings.repo_root`** change (managed from the SPA Settings page; see [SETTINGS.md](../SETTINGS.md)).

## Dashboards

1. **Blackbox** (when configured): `probe_success` and probe duration for the ready URL.
2. **Correlated app metrics** during degradation: `taskapi:http:5xx_ratio5m`, `rate(taskapi_db_pool_wait_count_total[5m])`, `taskapi:http:mutating_p99_seconds`.
3. **Build stamp** on each instance:

   ```promql
   max by (instance, version, revision) (taskapi_build_info)
   ```

## Direct checks (no Prometheus)

```bash
curl -sS -i "https://YOUR_HOST/health/ready"
```

- **`200`:** body should include `"status":"ok"` and per-check objects ([API-HTTP.md](../API-HTTP.md)).
- **`503`:** parse **`checks.database`** and **`checks.workspace_repo`** (when present) for `fail`.

## Logs (JSONL)

When the **database** check fails, **`readiness check failed`** at **Warn** uses **`operation`** **`health.ready`** and may include **`timeout_sec`** and **`deadline_exceeded`** for **`context.DeadlineExceeded`** (see [OBSERVABILITY.md](../OBSERVABILITY.md)).

```bash
rg '"operation":"health.ready"' /var/log/taskapi/*.log
```

## Mitigations

- **Postgres:** restore connectivity, relieve overload, or extend timeouts only as a temporary measure while fixing root cause.
- **Workspace repo:** ensure `app_settings.repo_root` (set from the SPA Settings page → gear icon) points at a directory that exists and is mounted on the pod/host; fix volume mounts or update the path from the UI / `PATCH /settings`. See [SETTINGS.md](../SETTINGS.md).
- After recovery, confirm probes green for several scrape intervals before declaring the incident resolved.
