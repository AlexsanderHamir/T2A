# Prometheus rules for T2A

| File | Purpose |
|------|---------|
| [`t2a-taskapi-rules.yaml`](./t2a-taskapi-rules.yaml) | Recording rules (`taskapi:http:5xx_ratio5m`, `taskapi:http:mutating_p99_seconds`, …) and **warning**-level alerts for `taskapi` HTTP metrics. |

## How to load

In `prometheus.yml`:

```yaml
rule_files:
  - /path/to/t2a-taskapi-rules.yaml
```

Reload or restart Prometheus. **Tune** `for` durations and thresholds (`0.02` 5xx ratio, `5` s p99, `200` in-flight, `5` pool waits/s) to match your [SLO table](../../docs/OBSERVABILITY.md#slis-and-slos-roadmap-b1).

Readiness alerting typically uses **blackbox** or platform probes; see commented example in the YAML and [docs/runbooks/alert-readiness.md](../../docs/runbooks/alert-readiness.md).
