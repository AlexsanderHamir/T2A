import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { SystemHealthResponse } from "@/types";
import { requestUrl } from "../test/requestUrl";
import { SystemStatusChip } from "./SystemStatusChip";

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

function makeHealth(overrides: Partial<SystemHealthResponse> = {}): SystemHealthResponse {
  return {
    build: { version: "v1.2.3", revision: "abc1234", go_version: "go1.23.0" },
    uptime_seconds: 60,
    now: "2026-04-19T12:00:00Z",
    http: {
      in_flight: 0,
      requests_total: 0,
      requests_by_class: { "2xx": 0, "3xx": 0, "4xx": 0, "5xx": 0, other: 0 },
      duration_seconds: { p50: 0, p95: 0, count: 0 },
    },
    sse: { subscribers: 1, dropped_frames_total: 0 },
    db_pool: {
      max_open_connections: 10,
      open_connections: 1,
      in_use_connections: 0,
      idle_connections: 1,
      wait_count_total: 0,
      wait_duration_seconds_total: 0,
    },
    agent: {
      queue_depth: 0,
      queue_capacity: 64,
      runs_total: 0,
      runs_by_terminal_status: { succeeded: 0, failed: 0, aborted: 0 },
      paused: false,
    },
    ...overrides,
  };
}

function renderChip(connected: boolean) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <SystemStatusChip connected={connected} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

function stubSystemHealth(payload: unknown) {
  vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
    const u = requestUrl(input as RequestInfo | URL);
    if (u.endsWith("/system/health")) {
      return jsonResponse(payload);
    }
    return new Response("not found", { status: 404 });
  });
}

describe("SystemStatusChip", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it("shows 'System unknown' while the health snapshot is still pending", () => {
    // Block fetch by never resolving so the query stays in pending.
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise<Response>(() => {}),
    );
    renderChip(true);
    const chip = screen.getByTestId("system-status-chip");
    expect(chip).toHaveAttribute("data-level", "unknown");
    expect(chip).toHaveTextContent(/System unknown/i);
  });

  it("renders OK with a live SSE dot when the snapshot is healthy and SSE is connected", async () => {
    stubSystemHealth(makeHealth());
    renderChip(true);

    const chip = await screen.findByTestId("system-status-chip");
    await waitFor(() => {
      expect(chip).toHaveAttribute("data-level", "ok");
    });
    expect(chip).toHaveAttribute("data-sse", "live");
    expect(chip).toHaveTextContent(/System OK/i);
    expect(chip).toHaveTextContent(/Live updates/i);
  });

  it("flips to 'Agent paused' when app_settings.agent_paused is true", async () => {
    stubSystemHealth(
      makeHealth({
        agent: {
          queue_depth: 0,
          queue_capacity: 64,
          runs_total: 0,
          runs_by_terminal_status: { succeeded: 0, failed: 0, aborted: 0 },
          paused: true,
        },
      }),
    );
    renderChip(true);

    const chip = await screen.findByTestId("system-status-chip");
    await waitFor(() => {
      expect(chip).toHaveAttribute("data-level", "paused");
    });
    expect(chip).toHaveTextContent(/Agent paused/i);
  });

  it("paused dominates degraded: even with 5xx responses, label reads 'Agent paused'", async () => {
    stubSystemHealth(
      makeHealth({
        http: {
          in_flight: 0,
          requests_total: 5,
          requests_by_class: { "2xx": 0, "3xx": 0, "4xx": 0, "5xx": 5, other: 0 },
          duration_seconds: { p50: 0, p95: 0, count: 5 },
        },
        agent: {
          queue_depth: 0,
          queue_capacity: 64,
          runs_total: 0,
          runs_by_terminal_status: { succeeded: 0, failed: 0, aborted: 0 },
          paused: true,
        },
      }),
    );
    renderChip(true);

    const chip = await screen.findByTestId("system-status-chip");
    await waitFor(() => {
      expect(chip).toHaveAttribute("data-level", "paused");
    });
    expect(chip).toHaveTextContent(/Agent paused/i);
  });

  it("renders Degraded when 5xx responses are present and the agent is not paused", async () => {
    stubSystemHealth(
      makeHealth({
        http: {
          in_flight: 0,
          requests_total: 5,
          requests_by_class: { "2xx": 0, "3xx": 0, "4xx": 0, "5xx": 5, other: 0 },
          duration_seconds: { p50: 0, p95: 0, count: 5 },
        },
      }),
    );
    renderChip(true);

    const chip = await screen.findByTestId("system-status-chip");
    await waitFor(() => {
      expect(chip).toHaveAttribute("data-level", "degraded");
    });
    expect(chip).toHaveTextContent(/Degraded/i);
  });

  it("shows 'Reconnecting' label and data-sse=down when the SSE stream is disconnected", async () => {
    stubSystemHealth(makeHealth());
    renderChip(false);

    const chip = await screen.findByTestId("system-status-chip");
    await waitFor(() => {
      expect(chip).toHaveAttribute("data-level", "ok");
    });
    expect(chip).toHaveAttribute("data-sse", "down");
    expect(chip).toHaveTextContent(/Reconnecting/i);
  });

  it("links to /observability so a click jumps to the explanation pane", () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise<Response>(() => {}),
    );
    renderChip(true);
    expect(screen.getByTestId("system-status-chip")).toHaveAttribute(
      "href",
      "/observability",
    );
  });
});
