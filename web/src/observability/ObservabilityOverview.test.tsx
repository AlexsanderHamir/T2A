import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { TaskStatsResponse } from "@/types/task";
import { ObservabilityOverview } from "./ObservabilityOverview";

function statsFixture(overrides: Partial<TaskStatsResponse> = {}): TaskStatsResponse {
  return {
    total: 18,
    ready: 5,
    critical: 3,
    scheduled: 4,
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

    const inventory = screen.getByLabelText("Task inventory");
    const cards = within(inventory).getAllByRole("article");
    expect(cards).toHaveLength(4);
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "true");
    });
    expect(screen.getByText("Loading the task table shape…")).toBeInTheDocument();
    expect(within(inventory).getByText("Loading breakdown…")).toBeInTheDocument();
  });

  it("renders unavailable state when stats settled to null", () => {
    render(<ObservabilityOverview stats={null} loading={false} />);

    const inventory = screen.getByLabelText("Task inventory");
    const cards = within(inventory).getAllByRole("article");
    expect(cards).toHaveLength(4);
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "false");
    });
    expect(within(inventory).getAllByText("—")).toHaveLength(4);
    expect(within(inventory).getByText("Breakdown unavailable")).toBeInTheDocument();
  });

  it("renders supporting inventory values from settled stats", () => {
    render(<ObservabilityOverview stats={statsFixture()} loading={false} />);

    expect(screen.getByTestId("obs-inventory-total")).toHaveTextContent("18");
    expect(screen.getByTestId("obs-inventory-done")).toHaveTextContent("6");
    expect(screen.getByTestId("obs-inventory-scheduled")).toHaveTextContent("4");
    expect(screen.getByTestId("obs-inventory-critical")).toHaveTextContent("3");
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

  it("Scheduled inventory: aria-busy while loading, then renders the count from stats.scheduled", () => {
    // Pending state: no stats yet, query in flight. Operators should
    // see the busy spinner on the new card just like every other KPI
    // — never a stale "0" that would falsely advertise an idle agent.
    const { rerender } = render(
      <ObservabilityOverview stats={undefined} loading={true} />,
    );
    const pendingCard = screen.getByTestId("obs-inventory-scheduled");
    expect(pendingCard).toHaveAttribute("aria-busy", "true");

    // Settled state: parser projects scheduled onto the response.
    // The card flips to aria-busy=false and surfaces the count.
    rerender(<ObservabilityOverview stats={statsFixture()} loading={false} />);
    const settledCard = screen.getByTestId("obs-inventory-scheduled");
    expect(settledCard).toHaveAttribute("aria-busy", "false");
    expect(settledCard).toHaveTextContent("4");
    expect(settledCard).toHaveTextContent(/deferred pickup/i);
  });

  it("Scheduled inventory: renders 0 when scheduled count is zero (no false 'awaiting' signal)", () => {
    render(
      <ObservabilityOverview
        stats={statsFixture({ scheduled: 0 })}
        loading={false}
      />,
    );
    const card = screen.getByTestId("obs-inventory-scheduled");
    expect(card).toHaveAttribute("aria-busy", "false");
    expect(card).toHaveTextContent("0");
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

    expect(screen.getByTestId("obs-inventory-total")).toHaveTextContent("0");
    expect(screen.getByTestId("obs-inventory-done")).toHaveTextContent("0");
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
