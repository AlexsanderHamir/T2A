# Grafana dashboards for T2A

| File | Purpose |
|------|---------|
| [`t2a-realtime-ux.json`](./t2a-realtime-ux.json) | Realtime UX SLO dashboard. Five top-row stat panels match the SLOs in [`docs/SLOs.md`](../../docs/SLOs.md); below them are drill-downs for `click_to_confirmed` latency, SSE fanout health, optimistic-mutation rollback, and Web Vitals. |

## How to import

1. Open Grafana → **Dashboards** → **New** → **Import**.
2. Upload `t2a-realtime-ux.json` (or paste it).
3. Select the Prometheus data source scraping `taskapi_*` metrics (the import dialog shows it as `${DS_PROMETHEUS}`).
4. Click **Import**.

The dashboard is self-contained and does not reference any Prometheus recording rules by name, so it will render even before [`../prometheus/t2a-taskapi-rules.yaml`](../prometheus/t2a-taskapi-rules.yaml) is loaded. The recording rules only exist so alert expressions stay cheap; the dashboard queries are raw `histogram_quantile` / `rate` against the source histograms.

## When a threshold changes

`docs/SLOs.md`, `../prometheus/t2a-taskapi-rules.yaml`, and the stat-panel thresholds in this JSON must agree. Grep for the current target value (e.g. `0.1` for the 100 ms click-to-confirmed bound) across all three files when you edit one.
