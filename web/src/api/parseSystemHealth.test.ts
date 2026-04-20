import { describe, expect, it } from "vitest";
import { parseSystemHealthResponse } from "./parseSystemHealth";

const minimalEnvelope = {
  build: { version: "v0.0.0", revision: "abcdef", go_version: "go1.23.0" },
  uptime_seconds: 0,
  now: "2026-04-19T12:00:00Z",
  http: {
    in_flight: 0,
    requests_total: 0,
    requests_by_class: { "2xx": 0, "3xx": 0, "4xx": 0, "5xx": 0, other: 0 },
    duration_seconds: { p50: 0, p95: 0, count: 0 },
  },
  sse: { subscribers: 0, dropped_frames_total: 0 },
  db_pool: {
    max_open_connections: 0,
    open_connections: 0,
    in_use_connections: 0,
    idle_connections: 0,
    wait_count_total: 0,
    wait_duration_seconds_total: 0,
  },
  agent: {
    queue_depth: 0,
    queue_capacity: 0,
    runs_total: 0,
    runs_by_terminal_status: { succeeded: 0, failed: 0, aborted: 0 },
    paused: false,
  },
};

describe("parseSystemHealthResponse", () => {
  it("accepts a well-formed empty envelope", () => {
    expect(parseSystemHealthResponse(minimalEnvelope)).toEqual(minimalEnvelope);
  });

  it("defaults agent.paused to false when the server omits the field (pre-4a backend)", () => {
    const noPaused: Record<string, unknown> = {
      ...minimalEnvelope,
      agent: {
        queue_depth: 0,
        queue_capacity: 0,
        runs_total: 0,
        runs_by_terminal_status: { succeeded: 0, failed: 0, aborted: 0 },
      },
    };
    const got = parseSystemHealthResponse(noPaused);
    expect(got.agent.paused).toBe(false);
  });

  it("preserves agent.paused=true when the server reports the operator pause flag", () => {
    const paused = {
      ...minimalEnvelope,
      agent: { ...minimalEnvelope.agent, paused: true },
    };
    const got = parseSystemHealthResponse(paused);
    expect(got.agent.paused).toBe(true);
  });

  it("accepts a fully-populated envelope", () => {
    const populated = {
      ...minimalEnvelope,
      uptime_seconds: 123.4,
      http: {
        in_flight: 3,
        requests_total: 1234,
        requests_by_class: { "2xx": 1200, "3xx": 0, "4xx": 30, "5xx": 4, other: 0 },
        duration_seconds: { p50: 0.012, p95: 0.18, count: 1234 },
      },
      sse: { subscribers: 4, dropped_frames_total: 7 },
      db_pool: {
        max_open_connections: 20,
        open_connections: 5,
        in_use_connections: 2,
        idle_connections: 3,
        wait_count_total: 1,
        wait_duration_seconds_total: 0.42,
      },
      agent: {
        queue_depth: 2,
        queue_capacity: 64,
        runs_total: 14,
        runs_by_terminal_status: { succeeded: 10, failed: 3, aborted: 1 },
      },
    };
    const got = parseSystemHealthResponse(populated);
    expect(got.uptime_seconds).toBe(123.4);
    expect(got.http.requests_by_class["2xx"]).toBe(1200);
    expect(got.http.duration_seconds.p95).toBeCloseTo(0.18);
    expect(got.agent.runs_by_terminal_status.succeeded).toBe(10);
  });

  it("seeds missing requests_by_class buckets to zero", () => {
    const partial = {
      ...minimalEnvelope,
      http: {
        in_flight: 0,
        requests_total: 5,
        // Only 2xx provided — backend should always seed all five but
        // the parser must degrade gracefully if a future client lands
        // before a backend update.
        requests_by_class: { "2xx": 5 },
        duration_seconds: { p50: 0, p95: 0, count: 0 },
      },
    };
    const got = parseSystemHealthResponse(partial);
    expect(got.http.requests_by_class).toEqual({
      "2xx": 5,
      "3xx": 0,
      "4xx": 0,
      "5xx": 0,
      other: 0,
    });
  });

  it("rejects unknown requests_by_class keys to catch contract drift", () => {
    expect(() =>
      parseSystemHealthResponse({
        ...minimalEnvelope,
        http: {
          ...minimalEnvelope.http,
          requests_by_class: { ...minimalEnvelope.http.requests_by_class, "1xx": 0 },
        },
      }),
    ).toThrow(/requests_by_class\.1xx/);
  });

  it("rejects unknown agent terminal status keys", () => {
    expect(() =>
      parseSystemHealthResponse({
        ...minimalEnvelope,
        agent: {
          ...minimalEnvelope.agent,
          runs_by_terminal_status: {
            ...minimalEnvelope.agent.runs_by_terminal_status,
            timed_out: 1,
          },
        },
      }),
    ).toThrow(/runs_by_terminal_status\.timed_out/);
  });

  it("seeds succeeded/failed/aborted to zero when only one provided", () => {
    const partial = {
      ...minimalEnvelope,
      agent: { ...minimalEnvelope.agent, runs_by_terminal_status: { succeeded: 7 } },
    };
    const got = parseSystemHealthResponse(partial);
    expect(got.agent.runs_by_terminal_status).toEqual({
      succeeded: 7,
      failed: 0,
      aborted: 0,
    });
  });

  it("rejects non-number numeric fields", () => {
    expect(() =>
      parseSystemHealthResponse({
        ...minimalEnvelope,
        uptime_seconds: "not-a-number",
      }),
    ).toThrow(/uptime_seconds/);
    expect(() =>
      parseSystemHealthResponse({
        ...minimalEnvelope,
        sse: { ...minimalEnvelope.sse, subscribers: null },
      }),
    ).toThrow(/sse\.subscribers/);
  });

  it("rejects non-object payloads", () => {
    expect(() => parseSystemHealthResponse(null)).toThrow(/system health payload/);
    expect(() => parseSystemHealthResponse("string")).toThrow(/system health payload/);
    expect(() => parseSystemHealthResponse([])).toThrow(/system health payload/);
  });

  it("rejects missing nested objects", () => {
    const noBuild: Record<string, unknown> = { ...minimalEnvelope };
    delete noBuild.build;
    expect(() => parseSystemHealthResponse(noBuild)).toThrow(/build/);
  });
});
