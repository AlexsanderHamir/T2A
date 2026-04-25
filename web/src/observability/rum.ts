/**
 * Real-User-Monitoring (RUM) beacon for the SPA.
 *
 * Why this lives here: docs/SLOs.md commits the product to four
 * latency/error SLOs (click→confirmed p95, mutation error rate,
 * optimistic rollback rate, SSE subscriber lag). The server-side
 * Prometheus counters cover the wire layer; this module produces the
 * client-side numerator+denominator series that those SLO rules
 * actually evaluate against.
 *
 * Transport choice: `navigator.sendBeacon` for visibility-change
 * flushes (it survives tab close), and the API transport wrapper for
 * the regular 10s flush so we keep nice error semantics during dev.
 * Both paths post to the same `/v1/rum` endpoint with the same JSON
 * shape documented in pkgs/tasks/handler/handler_rum.go.
 *
 * Backpressure: the in-memory queue is capped at `MAX_QUEUE_LENGTH`.
 * If the SPA enqueues faster than we can flush (e.g. user spamming
 * mutations on an offline laptop), the *oldest* events are dropped
 * — preserving recent events maximises the chance the bug or
 * behavior we are debugging is in the next batch the server sees.
 *
 * Test isolation: `__resetRUMForTests` is exported (vitest only —
 * production callers never reach for it) so the unit tests can run
 * deterministically without a transport leak between cases.
 */

import { sendRUMPayload } from "@/api/rum";

export type RUMMutationKind =
  | "task_create"
  | "task_patch"
  | "task_delete"
  | "task_requeue"
  | "checklist_add"
  | "checklist_edit"
  | "checklist_delete"
  | "subtask_create"
  | "settings_save";

export type RUMWebVitalName = "LCP" | "INP" | "CLS" | "FID" | "FCP" | "TTFB";

type RUMEvent =
  | { type: "mutation_started"; mutation_kind: RUMMutationKind }
  | {
      type: "mutation_optimistic_applied";
      mutation_kind: RUMMutationKind;
      duration_seconds: number;
    }
  | {
      type: "mutation_settled";
      mutation_kind: RUMMutationKind;
      duration_seconds: number;
      status_code: number;
    }
  | {
      type: "mutation_rolled_back";
      mutation_kind: RUMMutationKind;
      duration_seconds: number;
    }
  | { type: "sse_reconnected"; duration_seconds: number }
  | { type: "sse_resync_received" }
  | { type: "web_vitals"; name: RUMWebVitalName; value: number };

const FLUSH_INTERVAL_MS = 10_000;
const MAX_QUEUE_LENGTH = 200;
const RUM_ENDPOINT = "/v1/rum";

interface RumState {
  queue: RUMEvent[];
  flushTimer: ReturnType<typeof setInterval> | null;
  installed: boolean;
  visibilityHandler: (() => void) | null;
}

const state: RumState = {
  queue: [],
  flushTimer: null,
  installed: false,
  visibilityHandler: null,
};

function enqueue(event: RUMEvent): void {
  state.queue.push(event);
  if (state.queue.length > MAX_QUEUE_LENGTH) {
    // Drop oldest: a runaway user session shouldn't make the *next*
    // batch (which we're closer to flushing) lose its newest events.
    state.queue.splice(0, state.queue.length - MAX_QUEUE_LENGTH);
  }
}

function drainQueue(): RUMEvent[] {
  const events = state.queue;
  state.queue = [];
  return events;
}

function buildPayload(events: RUMEvent[]): string {
  return JSON.stringify({ events });
}

/**
 * flushNow flushes the in-memory queue to /v1/rum. Returns true if a
 * network request was attempted (queue had entries), false if there
 * was nothing to send. `useBeacon` is set by the visibilitychange
 * handler to use sendBeacon (survives tab close); the timer flush
 * uses fetch so we get a clean rejection during development.
 */
export function flushNow(useBeacon = false): boolean {
  if (state.queue.length === 0) {
    return false;
  }
  const events = drainQueue();
  const payload = buildPayload(events);

  if (useBeacon && typeof navigator !== "undefined" && typeof navigator.sendBeacon === "function") {
    try {
      const blob = new Blob([payload], { type: "application/json" });
      const ok = navigator.sendBeacon(RUM_ENDPOINT, blob);
      if (!ok) {
        // sendBeacon refused (queue full or payload too large); fall
        // back to fire-and-forget keepalive transport so we still try
        // to deliver during a tab-close race.
        void sendRUMPayload(payload, { keepalive: true }).catch(() => {
          /* swallow: best effort */
        });
      }
    } catch {
      /* swallow: RUM must never throw to the SPA */
    }
    return true;
  }

  void sendRUMPayload(payload).catch(() => {
    /* swallow: RUM is best-effort, never break user flow */
  });
  return true;
}

/**
 * installRUM wires the periodic flush timer and the
 * visibilitychange→sendBeacon path. Idempotent: calling twice is a
 * no-op (helpful in React-StrictMode dev where effects fire twice).
 * Tests call __resetRUMForTests between cases to undo the install.
 */
export function installRUM(): void {
  if (state.installed) {
    return;
  }
  state.installed = true;

  if (typeof document !== "undefined") {
    state.visibilityHandler = () => {
      if (document.visibilityState === "hidden") {
        flushNow(true);
      }
    };
    document.addEventListener("visibilitychange", state.visibilityHandler);
  }

  if (typeof window !== "undefined") {
    state.flushTimer = setInterval(() => {
      flushNow(false);
    }, FLUSH_INTERVAL_MS);
  }
}

/** Test-only reset hook. Removes the timer + visibility listener and
 * empties the queue so each test starts in a clean state. Production
 * callers never invoke this. */
export function __resetRUMForTests(): void {
  if (state.flushTimer !== null) {
    clearInterval(state.flushTimer);
    state.flushTimer = null;
  }
  if (state.visibilityHandler !== null && typeof document !== "undefined") {
    document.removeEventListener("visibilitychange", state.visibilityHandler);
    state.visibilityHandler = null;
  }
  state.installed = false;
  state.queue = [];
}

/** mutationStarted records the click moment for a mutation kind. */
export function mutationStarted(kind: RUMMutationKind): void {
  enqueue({ type: "mutation_started", mutation_kind: kind });
}

/** mutationOptimisticApplied records the click→optimistic-render
 * latency in milliseconds (we convert to seconds on the wire). */
export function mutationOptimisticApplied(kind: RUMMutationKind, durationMs: number): void {
  enqueue({
    type: "mutation_optimistic_applied",
    mutation_kind: kind,
    duration_seconds: clampDurationMs(durationMs) / 1000,
  });
}

/** mutationSettled records click→server-confirmed latency for the
 * given mutation kind. statusCode 0 means "network error / aborted"
 * — the server bucket label folds it into "unknown". */
export function mutationSettled(
  kind: RUMMutationKind,
  durationMs: number,
  statusCode: number,
): void {
  enqueue({
    type: "mutation_settled",
    mutation_kind: kind,
    duration_seconds: clampDurationMs(durationMs) / 1000,
    status_code: statusCode,
  });
}

/** mutationRolledBack records click→rollback latency for a mutation
 * whose optimistic apply was reverted on server error. */
export function mutationRolledBack(kind: RUMMutationKind, durationMs: number): void {
  enqueue({
    type: "mutation_rolled_back",
    mutation_kind: kind,
    duration_seconds: clampDurationMs(durationMs) / 1000,
  });
}

/** sseReconnected records an EventSource reconnect (browser auto or
 * post-resync). gapMs is the disconnect→reconnect latency. */
export function sseReconnected(gapMs: number): void {
  enqueue({ type: "sse_reconnected", duration_seconds: clampDurationMs(gapMs) / 1000 });
}

/** sseResyncReceived records that the SPA acted on a `resync`
 * directive from the hub. */
export function sseResyncReceived(): void {
  enqueue({ type: "sse_resync_received" });
}

/** webVital records a single web-vitals measurement. The web-vitals
 * package fires a callback per metric so call this once per metric
 * the app subscribes to. */
export function webVital(name: RUMWebVitalName, value: number): void {
  if (!Number.isFinite(value)) return;
  enqueue({ type: "web_vitals", name, value });
}

function clampDurationMs(ms: number): number {
  if (!Number.isFinite(ms) || ms < 0) return 0;
  // Server caps at 600s (validDurationSeconds in handler_rum.go);
  // mirror that here so we don't ship events the server will drop.
  if (ms > 600_000) return 600_000;
  return ms;
}

/** Test-only queue accessor (vitest assertions). */
export function __peekRUMQueueForTests(): readonly RUMEvent[] {
  return state.queue;
}
