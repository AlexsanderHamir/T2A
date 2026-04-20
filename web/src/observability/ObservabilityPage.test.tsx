import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { requestUrl } from "../test/requestUrl";
import { ObservabilityPage } from "./ObservabilityPage";

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

function renderPage() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, gcTime: 0 },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <ObservabilityPage />
    </QueryClientProvider>,
  );
}

describe("ObservabilityPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  const emptySystemHealth = {
    build: { version: "v0.0.0-test", revision: "test", go_version: "go1.23.0" },
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
    },
  };

  it("loads /tasks/stats and renders the overview", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const u = requestUrl(input);
      if (u.endsWith("/tasks/stats")) {
        return jsonResponse({
          total: 4,
          ready: 1,
          critical: 1,
          by_status: { ready: 1, done: 2, failed: 1 },
          by_priority: { medium: 3, critical: 1 },
          by_scope: { parent: 2, subtask: 2 },
          cycles: { by_status: {}, by_triggered_by: {} },
          phases: {
            by_phase_status: {
              diagnose: {},
              execute: {},
              verify: {},
              persist: {},
            },
          },
          runner: { by_runner: {}, by_model: {}, by_runner_model: {} },
          recent_failures: [],
        });
      }
      if (u.endsWith("/system/health")) {
        return jsonResponse(emptySystemHealth);
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();

    expect(
      await screen.findByRole("heading", { name: "Observability" }),
    ).toBeInTheDocument();

    await waitFor(() => {
      expect(screen.getByTestId("obs-kpi-total")).toHaveTextContent("4");
    });
    expect(screen.getByTestId("obs-kpi-done")).toHaveTextContent("2");
    expect(screen.getByTestId("obs-kpi-failed")).toHaveTextContent("1");
    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "System health" })).toBeInTheDocument();
    });
  });

  it("renders the unavailable state when the stats request fails", async () => {
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const u = requestUrl(input);
      if (u.endsWith("/tasks/stats")) {
        return new Response("boom", { status: 500 });
      }
      if (u.endsWith("/system/health")) {
        return new Response("boom", { status: 500 });
      }
      return new Response("not found", { status: 404 });
    });

    renderPage();

    await waitFor(() => {
      expect(screen.getAllByText("—").length).toBeGreaterThan(0);
    });
    expect(screen.getByText("Breakdown unavailable")).toBeInTheDocument();
    await waitFor(() => {
      expect(screen.getByText("System snapshot unavailable.")).toBeInTheDocument();
    });
  });
});
