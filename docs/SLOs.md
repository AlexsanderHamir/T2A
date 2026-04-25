# Service Level Objectives — T2A Realtime UX

Tracks the named realtime UX SLOs. Each SLO has a numeric target, a 30-day rolling window, and a documented data source so an operator can compute "are we burning budget?" without reverse-engineering the code.

Error budget for any SLO with target `T` is `1 − T`. A 99.5 % SLO has a 0.5 % budget; a 30-day window means roughly 3.6 hours of allowable "bad time" before the budget is exhausted.

## Conventions

- **Target**: minimum acceptable steady-state value. Numbers below it are SLO violations and consume error budget.
- **Window**: rolling 30 days unless otherwise noted.
- **Source**: which Prometheus / RUM metric drives the SLI; see [`pkgs/tasks/middleware/metrics_http.go`](../pkgs/tasks/middleware/metrics_http.go) and [`web/src/observability/rum.ts`](../web/src/observability/rum.ts).
- **Owner**: who pages first (one team for now: *T2A core*).

## SLOs

| Name | Target | Window | Source (numerator / denominator) | Why this number |
| --- | --- | --- | --- | --- |
| `slo_click_to_confirmed_p95_ms` | ≤ 100 ms | 30 d | `taskapi_rum_click_to_confirmed_seconds_bucket` (p95). "Confirmed" = optimistic render OR server 2xx, whichever fires first. | 100 ms is the perceptual "instant" threshold cited by Doherty / Nielsen; below it the user feels no lag. |
| `slo_sse_resync_rate` | ≤ 0.5 % | 30 d | `taskapi_sse_resync_emitted_total / taskapi_sse_publish_total` | Resyncs are correct (loss-free) but expensive (full cache drop + REST refetch). >0.5% means the ring buffer is too small or downstream clients are too slow. |
| `slo_sse_subscriber_lag_p99_seconds` | ≤ 2 s | 30 d | `taskapi_sse_subscriber_lag_seconds` (p99). Lag = `now − oldest pending frame timestamp` for any active subscriber. | The cross-tab "feels live" budget. Above 2 s users notice that their other tab is stale. |
| `slo_optimistic_rollback_rate` | ≤ 1 % | 30 d | RUM: `mutation_rolled_back / mutation_started`. | A high rollback rate means the optimistic apply is drifting from server validation — we predicted wrong too often. ≤1% lets the optimistic UX feel reliable. |
| `slo_mutation_error_rate` | ≤ 0.5 % | 30 d | RUM: `(mutation_settled_4xx + mutation_settled_5xx) / mutation_started`. | Independent of optimistic correctness — measures whether the mutation ultimately succeeded against the server. |

## Computation notes

- **Rates** are computed at evaluation time, NOT pre-aggregated, so a sudden burst of 100 % bad over 5 minutes is visible as a 5-minute spike before the 30-day window dilutes it. Use `rate(...[5m])` for short-window burn alerts, `rate(...[30d])` for the SLO compliance gauge.
- **p95 / p99 latency** SLIs read from histogram buckets (`histogram_quantile(0.95, ...)`). The buckets are tuned for `slo_click_to_confirmed_p95_ms` to give resolution between 25 ms and 1 s — see [`web/src/observability/rum.ts`](../web/src/observability/rum.ts) for the bucket constant.
- **`mutation_started` denominator** counts every mutation initiated by the SPA, including optimistic-applied mutations that later succeed. It does NOT count cache-only operations like `setQueryData` outside of a mutation hook.

## Burn-rate alerts (Phase 4d)

Multi-window multi-burn-rate per the Google SRE workbook:

- **Page**: 1 h burn rate of error budget > 14.4× (i.e. would exhaust 30-day budget in 50 minutes if sustained).
- **Ticket**: 6 h burn rate > 6× (would exhaust in 5 days if sustained).

Both expressed as `(rate(slo_violations[1h]) / (1 - SLO)) > 14.4`. Keep alert rules in your deployment system; this repo documents the signals and thresholds but no longer carries deploy-specific rule files.

## RUM transport

The frontend ships RUM events via the new `POST /v1/rum` endpoint (see [`pkgs/tasks/handler/handler_rum.go`](../pkgs/tasks/handler/handler_rum.go) and [`web/src/observability/rum.ts`](../web/src/observability/rum.ts)). Events are batched every 10 s and flushed via `navigator.sendBeacon` on `visibilitychange === 'hidden'` so a tab close does not lose the trailing batch. The endpoint is rate-limited per-IP and explicitly NOT covered by the SSE trigger surface (read-only side-effect: increments Prometheus counters; never publishes).

## Decisions worth recording

- **No nested labels for high-cardinality fields**. Mutation kind (`patch`, `delete`, `checklist_add`, …) is a label; `task_id` is NOT — that would explode cardinality in a long-running deployment. Per-task drill-down is left to the access log.
- **`mutation_optimistic_applied` is informational, not an SLI denominator**. It tells operators "X% of mutations actually felt instant," but it sits orthogonal to the rollback / error rate SLIs. Operators looking at the SLO panel see "mutation success" first; the optimistic-applied gauge sits next to it as a secondary KPI.
- **30-day rolling window, not calendar month**. The Grafana dashboard uses `last_over_time(...[30d])` so the SLO compliance display moves smoothly instead of resetting on the 1st of each month and lying about a freshly-burned budget.
- **Per-IP rate limiting on the RUM endpoint** (1 req/s burst 10) prevents a misbehaving SPA from amplifying a load incident into a metrics-storage bill.
