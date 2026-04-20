# Load + chaos tests for the realtime UX

These scripts validate Phase 5 of the plan at `.cursor/plans/production_realtime_smoothness_b17202b6.plan.md`. They intentionally do **not** run in CI — they need a long-lived target and a tuned infra (goroutine profiling, network shaping). Run them ad-hoc before flipping the `OptimisticMutationsEnabled` or `SSEReplayEnabled` flags.

## Layout

| File | Scenario | Target SLI |
|------|----------|------------|
| [`k6/scenario_a_fanout.js`](./k6/scenario_a_fanout.js) | 500 concurrent SSE subscribers + 50 PATCHes/sec for 30 min | `sse_resync_rate < 0.5%`, no goroutine leak |
| [`k6/scenario_b_cross_tab.js`](./k6/scenario_b_cross_tab.js) | 10 clients × 5 tabs each; each mutation must settle in every tab ≤ 2 s | `slo_sse_subscriber_lag_p99_seconds`, optimistic settle latency |
| [`chaos/scenario_c_kill_subscribers.sh`](./chaos/scenario_c_kill_subscribers.sh) | Kill 10 % of active subscriber TCP connections mid-publish | Ring + `Last-Event-ID` brings them back losslessly within 5 s |
| [`chaos/scenario_d_blackhole.sh`](./chaos/scenario_d_blackhole.sh) | 30 s network blackhole on the SSE port | `EventSource` reconnects, replays via `Last-Event-ID`, no UI desync |
| [`playwright/slow_network.spec.ts`](./playwright/slow_network.spec.ts) | Chrome with throttled CPU 4× and slow-3G; flip task statuses repeatedly | Optimistic render < 100 ms, eventual server settle < 2 s |

## Prerequisites

- `k6 >= 0.50` (the SSE extension is pulled via xk6; see `k6/README.md` for the exact build line).
- `iptables` or `pfctl` for chaos scripts (Linux / macOS).
- `@playwright/test` installed in the web workspace: `cd web && npm install --save-dev @playwright/test && npx playwright install chromium`.
- A taskapi instance at `$TASKAPI_ORIGIN` (default `http://127.0.0.1:8080`) with `pprof` exposed at `/debug/pprof` so the scripts can diff goroutine counts before/after.

## Running the full matrix

```bash
# 1. Terminal 1 — start the server with pprof.
go run ./cmd/taskapi --port 8080 --pprof

# 2. Terminal 2 — scenario A.
TASKAPI_ORIGIN=http://127.0.0.1:8080 k6 run ops/loadtest/k6/scenario_a_fanout.js

# 3. Terminal 3 — scenario B.
TASKAPI_ORIGIN=http://127.0.0.1:8080 k6 run ops/loadtest/k6/scenario_b_cross_tab.js

# 4. Terminal 4 — chaos C & D.
bash ops/loadtest/chaos/scenario_c_kill_subscribers.sh
sudo bash ops/loadtest/chaos/scenario_d_blackhole.sh   # needs root for pfctl/iptables

# 5. Terminal 5 — Playwright.
cd web && npx playwright test ops/loadtest/playwright/slow_network.spec.ts
```

A pass is:

1. All k6 `thresholds` green in the summary.
2. Goroutine count after scenario A within ±5 % of the pre-test count (see `thresholds.goroutines_delta` in `scenario_a_fanout.js`).
3. No `resync` frames appear after chaos C / D reconnects that correspond to events already seen by the client (the `Last-Event-ID` path should cover them).
4. Playwright reports `optimisticRenderMs < 100` and `serverSettleMs < 2000` for every tracked mutation.

See [`docs/SLOs.md`](../../docs/SLOs.md) for the canonical SLI definitions these thresholds track.
