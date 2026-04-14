# Runbook: Readiness / `GET /health/ready`

## Note

**`taskapi` metrics** intentionally omit health paths from the HTTP latency histogram; **readiness** is usually monitored with a **synthetic probe** (Prometheus **blackbox_exporter**, Kubernetes readiness, or a load balancer health check).

## Example

Uncomment and adapt the `TaskAPIReadinessProbeFailing` alert in [`deploy/prometheus/t2a-taskapi-rules.yaml`](../../deploy/prometheus/t2a-taskapi-rules.yaml) once `probe_success` (or equivalent) exists.

## Check first

1. **Response body:** `503` with `checks.database` / `workspace_repo` ([API-HTTP.md](../API-HTTP.md)).
2. **Logs:** `operation` **`health.ready`** at **Warn** with **`deadline_exceeded`** when the DB ping times out.

## Mitigations

- Restore Postgres connectivity; fix **`REPO_ROOT`** path if `workspace_repo` fails; scale DB if timeouts are load-related.
