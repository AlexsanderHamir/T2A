import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  __peekRUMQueueForTests,
  __resetRUMForTests,
  flushNow,
  installRUM,
  mutationOptimisticApplied,
  mutationRolledBack,
  mutationSettled,
  mutationStarted,
  sseReconnected,
  sseResyncReceived,
  webVital,
} from "./rum";

describe("rum module", () => {
  let fetchMock: ReturnType<typeof vi.fn>;
  let beaconMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    fetchMock = vi.fn(() => Promise.resolve(new Response(null, { status: 204 })));
    vi.stubGlobal("fetch", fetchMock);
    beaconMock = vi.fn(() => true);
    Object.defineProperty(navigator, "sendBeacon", {
      configurable: true,
      writable: true,
      value: beaconMock,
    });
    __resetRUMForTests();
  });

  afterEach(() => {
    __resetRUMForTests();
    vi.unstubAllGlobals();
  });

  // The queue must accumulate calls in publish order so the wire
  // payload preserves the click→optimistic→settled chronology that
  // the server-side histograms depend on. A subtle bug where a
  // helper bypassed `enqueue` (e.g. ran fetch synchronously) would
  // re-order events and break the SLO numerator.
  it("accumulates events in publish order until flushed", () => {
    mutationStarted("task_patch");
    mutationOptimisticApplied("task_patch", 12);
    mutationSettled("task_patch", 87, 200);
    expect(__peekRUMQueueForTests()).toHaveLength(3);
    expect(__peekRUMQueueForTests()[0]).toMatchObject({ type: "mutation_started" });
    expect(__peekRUMQueueForTests()[2]).toMatchObject({
      type: "mutation_settled",
      status_code: 200,
    });
  });

  // The fetch-based flush path is what the periodic 10s timer uses
  // in production. We pin (a) it POSTs JSON to /v1/rum, (b) it
  // empties the queue so the next batch starts fresh, and (c) the
  // returned boolean reflects whether work was attempted — the
  // visibility handler relies on that signal to skip empty flushes.
  it("flushNow posts JSON to /v1/rum and clears the queue", () => {
    mutationStarted("task_patch");
    const sent = flushNow(false);
    expect(sent).toBe(true);
    expect(__peekRUMQueueForTests()).toHaveLength(0);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [url, init] = fetchMock.mock.calls[0] as [string, RequestInit];
    expect(url).toBe("/v1/rum");
    expect(init.method).toBe("POST");
    const body = JSON.parse(init.body as string) as { events: unknown[] };
    expect(body.events).toHaveLength(1);
  });

  // visibilitychange flushes use sendBeacon — the only transport
  // that survives tab close. If a future refactor accidentally falls
  // back to fetch on the hidden path, every batch the user is in
  // the middle of typing would be lost on tab close, silently
  // breaking the SLO denominators for that session.
  it("flushNow with useBeacon=true uses navigator.sendBeacon", () => {
    sseResyncReceived();
    flushNow(true);
    expect(beaconMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  // No queue → no network call, regardless of transport. This pins
  // the contract the visibility handler depends on (it always calls
  // flushNow(true) on tab hide; we don't want a beacon every hide).
  it("flushNow returns false when the queue is empty and never calls fetch", () => {
    expect(flushNow(false)).toBe(false);
    expect(flushNow(true)).toBe(false);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(beaconMock).not.toHaveBeenCalled();
  });

  // installRUM is idempotent — under React StrictMode the install
  // effect fires twice and we don't want to double-register the
  // visibility listener (which would double the beacon traffic on
  // every tab hide).
  it("installRUM is idempotent across repeated calls", () => {
    const addSpy = vi.spyOn(document, "addEventListener");
    installRUM();
    installRUM();
    installRUM();
    const visibilityCalls = addSpy.mock.calls.filter((c) => c[0] === "visibilitychange");
    expect(visibilityCalls).toHaveLength(1);
  });

  // The visibility handler only flushes on hidden — pinning this
  // prevents a spurious beacon on every tab focus event.
  it("visibilitychange→hidden triggers a sendBeacon flush", () => {
    installRUM();
    mutationStarted("task_patch");
    Object.defineProperty(document, "visibilityState", {
      configurable: true,
      get: () => "hidden",
    });
    document.dispatchEvent(new Event("visibilitychange"));
    expect(beaconMock).toHaveBeenCalledTimes(1);
  });

  // Hostile inputs (NaN, negative, infinity) MUST NOT pollute the
  // histograms — the server validates again, but client clamping
  // means we don't waste a wire byte on events the server will drop.
  it("clamps invalid durations to 0 so the histogram stays clean", () => {
    mutationOptimisticApplied("task_patch", Number.NaN);
    mutationSettled("task_patch", -100, 200);
    mutationRolledBack("task_patch", Number.POSITIVE_INFINITY);
    sseReconnected(-1);
    const events = __peekRUMQueueForTests();
    expect(events).toHaveLength(4);
    for (const ev of events) {
      if ("duration_seconds" in ev) {
        expect(ev.duration_seconds).toBe(0);
      }
    }
  });

  // The 200-event queue cap protects against a runaway loop in the
  // SPA flooding the server. We pin that overflow drops *oldest*
  // events, preserving the most recent ones for debugging.
  it("drops oldest events past the queue cap", () => {
    for (let i = 0; i < 250; i++) {
      mutationStarted("task_patch");
    }
    expect(__peekRUMQueueForTests()).toHaveLength(200);
  });

  // Web vitals: skipping non-finite values is the documented
  // contract (the web-vitals lib can fire NaN during page lifecycle
  // edge cases like a freeze→resume).
  it("webVital skips non-finite values", () => {
    webVital("LCP", Number.NaN);
    webVital("LCP", 1234);
    expect(__peekRUMQueueForTests()).toHaveLength(1);
    expect(__peekRUMQueueForTests()[0]).toMatchObject({ type: "web_vitals", value: 1234 });
  });
});
