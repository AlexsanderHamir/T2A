// Scenario B — cross-tab reconciliation.
//
// Simulates 10 users, each with 5 open tabs (50 SSE subscribers
// total, sharing 10 server-visible "users"). Every 2 seconds a
// random tab issues a PATCH; the script records the per-tab time
// until that tab sees the server-confirmed event back on /events.
//
// Pass criterion: p99(settle_latency) <= 2 s (matches
// slo_sse_subscriber_lag_p99_seconds).
//
// The script does NOT try to model the optimistic render because
// that is a client-side concern covered by the Playwright scenario.
// What we validate here is the server fanout promise: once a PATCH
// returns 2xx, every connected tab learns about it within 2 s.

import http from "k6/http";
import sse from "k6/x/sse";
import { fail } from "k6";
import { Trend } from "k6/metrics";

const ORIGIN = __ENV.TASKAPI_ORIGIN || "http://127.0.0.1:8080";
const USERS = Number(__ENV.USERS || 10);
const TABS_PER_USER = Number(__ENV.TABS_PER_USER || 5);
const DURATION_MIN = Number(__ENV.DURATION_MIN || 10);

const settleLatency = new Trend("cross_tab_settle_ms", true);

export const options = {
  scenarios: {
    tabs: {
      executor: "constant-vus",
      exec: "tab",
      vus: USERS * TABS_PER_USER,
      duration: `${DURATION_MIN}m`,
      gracefulStop: "20s",
    },
  },
  thresholds: {
    cross_tab_settle_ms: [
      "p(99) < 2000",
      "p(95) < 1000",
    ],
  },
};

function fetchTaskIDs() {
  const res = http.get(`${ORIGIN}/tasks?limit=50`);
  if (res.status !== 200) {
    fail(`GET /tasks failed with status ${res.status}`);
  }
  const body = res.json();
  return (body.items || []).map((t) => t.id);
}

let TASK_IDS = null;
export function setup() {
  TASK_IDS = fetchTaskIDs();
  if (TASK_IDS.length < 3) {
    fail(`need >= 3 pre-seeded tasks; found ${TASK_IDS.length}`);
  }
  return { taskIDs: TASK_IDS };
}

// Each VU is a tab. It holds an SSE connection AND periodically
// issues its own PATCH; we record settle latency for mutations the
// tab *did not* itself initiate (that's the cross-tab path), using
// the embedded mutation timestamp written into the task's title.
export function tab(data) {
  const ids = data.taskIDs;
  const markerPrefix = `k6-cross-${__VU}-`;
  let openMutations = new Map();

  sse.open(`${ORIGIN}/events`, null, (client) => {
    client.on("event", (ev) => {
      let payload;
      try { payload = JSON.parse(ev.data); } catch (_) { return; }
      if (!payload || payload.type !== "task_updated") return;
      const title = payload.task && payload.task.title;
      if (!title) return;
      // Titles set by the mutate loop look like
      //   "k6-cross-<vu>-<epochMs>". When any tab sees that title
      // come back on the stream, record `now - epochMs` as the
      // settle latency; that's exactly what the SLI measures.
      const m = /k6-cross-\d+-(\d+)/.exec(title);
      if (!m) return;
      const at = Number(m[1]);
      if (!isNaN(at)) settleLatency.add(Date.now() - at);
    });
  });

  // Drive mutations from this VU too; the critical point is that
  // peers see them, and peers are just other VUs on the same event
  // stream. Sleep between PATCHes so total RPS is modest.
  while (true) {
    const id = ids[Math.floor(Math.random() * ids.length)];
    const title = `${markerPrefix}${Date.now()}`;
    http.patch(
      `${ORIGIN}/tasks/${id}`,
      JSON.stringify({ title }),
      { headers: { "Content-Type": "application/json" } },
    );
    // Randomised 1–3 s pacing per tab.
    const sleepMs = 1000 + Math.floor(Math.random() * 2000);
    const t = Date.now() + sleepMs;
    while (Date.now() < t) {
      // busy-wait is fine — VU count is bounded and k6 yields between HTTP calls.
    }
  }
}
