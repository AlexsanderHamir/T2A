import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { TaskStatsResponse } from "@/types/task";
import { ObservabilityOverview } from "./ObservabilityOverview";

function statsFixture(overrides: Partial<TaskStatsResponse> = {}): TaskStatsResponse {
  return {
    total: 18,
    ready: 5,
    critical: 3,
    by_status: {
      ready: 5,
      running: 2,
      blocked: 1,
      review: 1,
      done: 6,
      failed: 3,
    },
    by_priority: {
      low: 4,
      medium: 6,
      high: 5,
      critical: 3,
    },
    by_scope: { parent: 11, subtask: 7 },
    ...overrides,
  } as TaskStatsResponse;
}

describe("ObservabilityOverview", () => {
  it("renders skeletons while loading and no stats are available yet", () => {
    render(<ObservabilityOverview stats={undefined} loading={true} />);

    const counters = screen.getByLabelText("Headline counters");
    const cards = within(counters).getAllByRole("article");
    expect(cards).toHaveLength(6);
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "true");
    });
    // Loading caption from totalMeta.
    expect(within(counters).getByText("Loading breakdown…")).toBeInTheDocument();
  });

  it("renders unavailable state when stats settled to null", () => {
    render(<ObservabilityOverview stats={null} loading={false} />);

    const counters = screen.getByLabelText("Headline counters");
    const cards = within(counters).getAllByRole("article");
    expect(cards).toHaveLength(6);
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "false");
    });
    expect(within(counters).getAllByText("—")).toHaveLength(6);
    expect(within(counters).getByText("Breakdown unavailable")).toBeInTheDocument();
  });

  it("renders all six headline KPI values from settled stats", () => {
    render(<ObservabilityOverview stats={statsFixture()} loading={false} />);

    expect(screen.getByTestId("obs-kpi-total")).toHaveTextContent("18");
    expect(screen.getByTestId("obs-kpi-done")).toHaveTextContent("6");
    expect(screen.getByTestId("obs-kpi-failed")).toHaveTextContent("3");
    // running 2 + blocked 1 + review 1 = 4
    expect(screen.getByTestId("obs-kpi-in-flight")).toHaveTextContent("4");
    expect(screen.getByTestId("obs-kpi-ready")).toHaveTextContent("5");
    expect(screen.getByTestId("obs-kpi-critical")).toHaveTextContent("3");
    expect(
      screen.getByText("11 parent • 7 subtasks"),
    ).toBeInTheDocument();
  });

  it("uses singular 'subtask' when by_scope.subtask is 1", () => {
    render(
      <ObservabilityOverview
        stats={statsFixture({ by_scope: { parent: 4, subtask: 1 } })}
        loading={false}
      />,
    );
    expect(screen.getByText("4 parent • 1 subtask")).toBeInTheDocument();
  });

  it("renders status segments proportional to their counts", () => {
    render(<ObservabilityOverview stats={statsFixture()} loading={false} />);

    // Total status sum = 18; failed = 3 → ~16.67%.
    const failedSeg = screen.getByTestId("obs-bar-segment-failed");
    expect(failedSeg.style.width).toMatch(/16\./);
    // Done = 6 → ~33.33%.
    const doneSeg = screen.getByTestId("obs-bar-segment-done");
    expect(doneSeg.style.width).toMatch(/33\./);
    // Tooltip carries the exact count + percentage.
    expect(failedSeg).toHaveAttribute("title", "Failed: 3 (17%)");
  });

  it("renders priority segments and labels (escalation order)", () => {
    render(<ObservabilityOverview stats={statsFixture()} loading={false} />);
    const priority = screen.getByLabelText("Priority distribution");
    const legend = within(priority).getByLabelText(
      "Priority distribution legend",
    );
    const items = within(legend).getAllByRole("listitem");
    expect(items.map((i) => i.textContent)).toEqual([
      "Low4",
      "Medium6",
      "High5",
      "Critical3",
    ]);
  });

  it("renders the scope donut with parent/subtask arcs", () => {
    render(<ObservabilityOverview stats={statsFixture()} loading={false} />);
    expect(screen.getByTestId("obs-donut-arc-parent")).toBeInTheDocument();
    expect(screen.getByTestId("obs-donut-arc-subtask")).toBeInTheDocument();
    const scope = screen.getByLabelText("Scope");
    expect(within(scope).getByLabelText(/Scope: Parent 11.*Subtask 7/)).toBeInTheDocument();
  });

  it("shows empty placeholders when stats are zeroed out", () => {
    const empty = statsFixture({
      total: 0,
      ready: 0,
      critical: 0,
      by_status: {},
      by_priority: {},
      by_scope: { parent: 0, subtask: 0 },
    });
    render(<ObservabilityOverview stats={empty} loading={false} />);

    expect(screen.getByTestId("obs-kpi-total")).toHaveTextContent("0");
    expect(screen.getByTestId("obs-kpi-failed")).toHaveTextContent("0");
    // Distribution captions surface the empty case.
    expect(
      screen.getAllByText("No tasks recorded yet").length,
    ).toBeGreaterThanOrEqual(2);
    // The chart still renders its row (no segments) — accessible name reflects the empty state.
    const status = screen.getByLabelText("Status distribution");
    expect(
      within(status).getByLabelText("Status distribution: no data yet"),
    ).toBeInTheDocument();
  });
});
