import { render, screen, within } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { TaskStatsResponse } from "@/types";
import { TaskListStatsStrip } from "./TaskListStatsStrip";

const isUiFeatureOmitted = vi.hoisted(() => vi.fn((_feature: string) => false));

vi.mock("@/launch/omittedFeatures", () => ({
  isUiFeatureOmitted: (feature: string) => isUiFeatureOmitted(feature),
}));

function makeStats(
  overrides: Partial<TaskStatsResponse> = {},
): TaskStatsResponse {
  return {
    total: 0,
    ready: 0,
    critical: 0,
    scheduled: 0,
    by_status: {},
    by_priority: {},
    cycles: { by_status: {}, by_triggered_by: {} },
    phases: {
      by_phase_status: {
        execute: {},
        verify: {},
      },
    },
    runner: {
      by_runner: {},
      by_model: {},
      by_runner_model: {},
      by_runner_model_resolved: {},
    },
    recent_failures: [],
    ...overrides,
  };
}

describe("TaskListStatsStrip", () => {
  beforeEach(() => {
    isUiFeatureOmitted.mockImplementation(() => false);
  });

  it("renders nothing when stats is null", () => {
    render(<TaskListStatsStrip stats={null} />);
    expect(
      screen.queryByTestId("task-list-stats-strip"),
    ).not.toBeInTheDocument();
  });

  it("renders nothing on a fresh database (total = 0)", () => {
    render(<TaskListStatsStrip stats={makeStats()} />);
    expect(
      screen.queryByTestId("task-list-stats-strip"),
    ).not.toBeInTheDocument();
  });

  it("renders total + ready when total > 0 (always-on baseline)", () => {
    render(
      <TaskListStatsStrip stats={makeStats({ total: 4, ready: 2 })} />,
    );
    const strip = screen.getByTestId("task-list-stats-strip");
    expect(within(strip).getByText("Total")).toBeInTheDocument();
    expect(within(strip).getByText("Ready")).toBeInTheDocument();
    expect(screen.getByTestId("task-list-stats-total")).toHaveTextContent("4");
    expect(screen.getByTestId("task-list-stats-ready")).toHaveTextContent("2");
    expect(
      screen.queryByTestId("task-list-stats-critical"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByTestId("task-list-stats-scheduled"),
    ).not.toBeInTheDocument();
  });

  it("renders critical / scheduled / review / blocked pills only when non-zero", () => {
    render(
      <TaskListStatsStrip
        stats={makeStats({
          total: 9,
          ready: 3,
          critical: 1,
          scheduled: 2,
          by_status: { review: 1, blocked: 2, ready: 3, done: 1 },
        })}
      />,
    );
    expect(screen.getByTestId("task-list-stats-critical")).toHaveTextContent(
      "1",
    );
    expect(screen.getByTestId("task-list-stats-scheduled")).toHaveTextContent(
      "2",
    );
    expect(screen.getByTestId("task-list-stats-review")).toHaveTextContent(
      "1",
    );
    expect(screen.getByTestId("task-list-stats-blocked")).toHaveTextContent(
      "2",
    );
    const strip = screen.getByTestId("task-list-stats-strip");
    expect(within(strip).getByText("Scheduled")).toBeInTheDocument();
    expect(within(strip).getByText("Review")).toBeInTheDocument();
    expect(within(strip).getByText("Blocked")).toBeInTheDocument();
  });

  it("hides the scheduled pill when launch omits schedule", () => {
    isUiFeatureOmitted.mockImplementation((feature) => feature === "schedule");
    render(
      <TaskListStatsStrip
        stats={makeStats({
          total: 9,
          ready: 3,
          scheduled: 2,
        })}
      />,
    );
    expect(
      screen.queryByTestId("task-list-stats-scheduled"),
    ).not.toBeInTheDocument();
  });
});
