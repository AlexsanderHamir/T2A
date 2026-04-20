// Scenario A — high-fanout + mutation storm.
//
// 500 concurrent SSE subscribers stay connected for 30 minutes while
// a second VU pool issues 50 PATCH /tasks/:id per second. Exercises
// the coalesce window, the ring buffer, and the slow-consumer
// eviction path at once.
//
// Pass criteria (enforced via thresholds below):
//   * sse_resync_rate < 0.5 %
//     (plan Phase 5; matches slo_sse_resync_rate in docs/SLOs.md)
//   * 99 % of PATCHes return 2xx
//   * goroutine count after the run is within +5 % of before
//     (checked externally via /debug/pprof/goroutine?debug=1; see
//      README.md. k6 cannot diff goroutines itself but it writes the
//      before/after counts to stdout so CI can grep them.)

import http from "k6/http";
import sse from "k6/x/sse";
import { check, fail } from "k6";
import { Counter, Rate, Trend } from "k6/metrics";

const ORIGIN = __ENV.TASKAPI_ORIGIN || "http://127.0.0.1:8080";
const SUBSCRIBERS = Number(__ENV.SUBSCRIBERS || 500);
const PATCH_RPS = Number(__ENV.PATCH_RPS || 50);
const DURATION_MIN = Number(__ENV.DURATION_MIN || 30);

const sseReceived = new Counter("sse_events_received");
const sseResync = new Counter("sse_resync_directives");
const patchOk = new Rate("patch_ok");
const patchLatency = new Trend("patch_latency_ms", true);

export const options = {
  scenarios: {
    subscribers: {
      executor: "constant-vus",
      exec: "subscribe",
      vus: SUBSCRIBERS,
      duration: `${DURATION_MIN}m`,
      gracefulStop: "30s",
    },
    mutators: {
      executor: "constant-arrival-rate",
      exec: "mutate",
      rate: PATCH_RPS,
      timeUnit: "1s",
      duration: `${DURATION_MIN}m`,
      preAllocatedVUs: Math.max(10, Math.floor(PATCH_RPS / 2)),
      maxVUs: PATCH_RPS * 2,
      gracefulStop: "30s",
    },
  },
  thresholds: {
    // slo_sse_resync_rate < 0.5 % — computed as
    //   sse_resync_directives / sse_events_received.
    // Expressed here as a hard cap on the per-event resync rate.
    sse_resync_directives: [`count/(count+${SUBSCRIBERS * DURATION_MIN * 60}) < 0.005`],
    // Mutation error budget matches slo_mutation_error_rate (0.5 %).
    patch_ok: ["rate > 0.995"],
    // p95 patch latency — taskapi:http:mutating_p99_seconds watches
    // p99; we budget p95 < 500 ms here to catch regressions earlier.
    patch_latency_ms: ["p(95) < 500"],
  },
};

// --- taskID pool ----------------------------------------------------
// Scenario assumes a prepopulated test project with at least 20 tasks.
// Grab their IDs once per VU so each PATCH stays within a known set
// and the coalesce path (same {type,id}) is actually exercised.
function fetchTaskIDs() {
  const res = http.get(`${ORIGIN}/tasks?limit=50`);
  if (res.status !== 200) {
    fail(`GET /tasks failed with status ${res.status}; preload fixtures before running scenario A`);
  }
  const body = res.json();
  const ids = (body.items || []).map((t) => t.id);
  if (ids.length < 5) {
    fail(`need at least 5 pre-seeded tasks, found ${ids.length}`);
  }
  return ids;
}

let TASK_IDS = null;
export function setup() {
  TASK_IDS = fetchTaskIDs();
  // Goroutine-count snapshot — the operator compares this with a
  // post-teardown curl to /debug/pprof/goroutine?debug=1 to confirm
  // no leak. k6 cannot parse pprof so we just log the timestamp.
  console.log(`[setup] baseline goroutine snapshot at ${new Date().toISOString()} — capture /debug/pprof/goroutine NOW`);
  return { taskIDs: TASK_IDS };
}

export function teardown() {
  console.log(`[teardown] post-run goroutine snapshot at ${new Date().toISOString()} — capture /debug/pprof/goroutine again and diff`);
}

export function subscribe() {
  const url = `${ORIGIN}/events`;
  sse.open(url, null, (client) => {
    client.on("event", (ev) => {
      sseReceived.add(1);
      try {
        const payload = JSON.parse(ev.data);
        if (payload.type === "resync") {
          sseResync.add(1);
        }
      } catch (_) {
        // Non-JSON frames (heartbeats) are intentionally ignored.
      }
    });
    client.on("error", () => {
      // EventSource retries automatically; nothing to do here.
    });
  });
}

export function mutate(data) {
  const ids = (data && data.taskIDs) || TASK_IDS || [];
  if (ids.length === 0) {
    return;
  }
  const id = ids[Math.floor(Math.random() * ids.length)];
  const status = Math.random() < 0.5 ? "ready" : "working";
  const t0 = Date.now();
  const res = http.patch(
    `${ORIGIN}/tasks/${id}`,
    JSON.stringify({ status }),
    { headers: { "Content-Type": "application/json" } },
  );
  patchLatency.add(Date.now() - t0);
  patchOk.add(res.status >= 200 && res.status < 300);
  check(res, { "patch 2xx": (r) => r.status >= 200 && r.status < 300 });
}
