import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { SystemHealthResponse } from "@/types";
import { ObservabilitySystem } from "./ObservabilitySystem";

const baseHealth: SystemHealthResponse = {
  build: { version: "v1.2.3", revision: "abc1234", go_version: "go1.23.0" },
  uptime_seconds: 3600,
  now: "2026-04-19T12:00:00Z",
  http: {
    in_flight: 2,
    requests_total: 1000,
    requests_by_class: { "2xx": 980, "3xx": 0, "4xx": 15, "5xx": 5, other: 0 },
    duration_seconds: { p50: 0.012, p95: 0.18, count: 1000 },
  },
  sse: { subscribers: 3, dropped_frames_total: 0 },
  db_pool: {
    max_open_connections: 20,
    open_connections: 5,
    in_use_connections: 1,
    idle_connections: 4,
    wait_count_total: 0,
    wait_duration_seconds_total: 0,
  },
  agent: {
    queue_depth: 0,
    queue_capacity: 64,
    runs_total: 12,
    runs_by_terminal_status: { succeeded: 10, failed: 2, aborted: 0 },
    paused: false,
  },
};

describe("ObservabilitySystem", () => {
  it("renders skeleton KPIs while the snapshot is loading", () => {
    const { container } = render(
      <ObservabilitySystem health={undefined} loading={true} />,
    );
    expect(screen.getByText("Loading status…")).toBeInTheDocument();
    // KPI grid renders with skeletons; the `aria-busy` attribute on the
    // KpiCard root is the documented signal for loading.
    const busyCards = container.querySelectorAll('[aria-busy="true"]');
    expect(busyCards.length).toBeGreaterThanOrEqual(6);
  });

  it("renders the unavailable state when the snapshot fetch settled to null", () => {
    render(<ObservabilitySystem health={null} loading={false} />);
    expect(screen.getByText("Status unavailable.")).toBeInTheDocument();
    // Each KPI value collapses to "—" so the layout doesn't shift.
    const dashes = screen.getAllByText("—");
    expect(dashes.length).toBeGreaterThanOrEqual(6);
  });

  it("renders KPIs, distributions, and the build footer when populated", () => {
    render(<ObservabilitySystem health={baseHealth} loading={false} />);

    expect(screen.getByTestId("obs-system-kpi-in-flight")).toHaveTextContent("2");
    expect(screen.getByTestId("obs-system-kpi-requests")).toHaveTextContent(
      /1,000|1000/,
    );
    expect(screen.getByTestId("obs-system-kpi-sse-subs")).toHaveTextContent("3");
    expect(screen.getByTestId("obs-system-kpi-sse-dropped")).toHaveTextContent("0");
    expect(screen.getByTestId("obs-system-kpi-db-in-use")).toHaveTextContent("1");
    expect(screen.getByTestId("obs-system-kpi-agent-queue")).toHaveTextContent("0");

    expect(
      screen.getByRole("img", { name: /HTTP responses by class/ }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("img", { name: /Agent runs by outcome/ }),
    ).toBeInTheDocument();

    const footer = screen.getByLabelText("Build and uptime");
    expect(within(footer).getByText("v1.2.3")).toBeInTheDocument();
    expect(within(footer).getByText("abc1234")).toBeInTheDocument();
    expect(within(footer).getByText("go1.23.0")).toBeInTheDocument();
    expect(within(footer).getByText("1 h")).toBeInTheDocument();
  });

  it("flags the pane as degraded when 5xx responses appear", () => {
    const { container } = render(
      <ObservabilitySystem health={baseHealth} loading={false} />,
    );
    const pane = container.querySelector(".obs-system");
    expect(pane?.className).toContain("obs-system--degraded");
    expect(screen.getByText(/5 5xx responses/)).toBeInTheDocument();
  });

  it("flags the pane as ok when no failure signals are present", () => {
    const clean: SystemHealthResponse = {
      ...baseHealth,
      http: {
        ...baseHealth.http,
        requests_by_class: {
          "2xx": 1000,
          "3xx": 0,
          "4xx": 0,
          "5xx": 0,
          other: 0,
        },
      },
      sse: { subscribers: 3, dropped_frames_total: 0 },
      agent: {
        ...baseHealth.agent,
        runs_by_terminal_status: { succeeded: 12, failed: 0, aborted: 0 },
      },
    };
    const { container } = render(
      <ObservabilitySystem health={clean} loading={false} />,
    );
    const pane = container.querySelector(".obs-system");
    expect(pane?.className).toContain("obs-system--ok");
    expect(screen.getByText(/Build v1\.2\.3 .* up for 1 h/)).toBeInTheDocument();
  });
});
