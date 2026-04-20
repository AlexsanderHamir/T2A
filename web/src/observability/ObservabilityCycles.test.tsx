import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import type { TaskStatsResponse } from "@/types/task";
import { ObservabilityCycles } from "./ObservabilityCycles";

function emptyStats(overrides: Partial<TaskStatsResponse> = {}): TaskStatsResponse {
  return {
    total: 0,
    ready: 0,
    critical: 0,
    by_status: {},
    by_priority: {},
    by_scope: { parent: 0, subtask: 0 },
    cycles: { by_status: {}, by_triggered_by: {} },
    phases: {
      by_phase_status: {
        diagnose: {},
        execute: {},
        verify: {},
        persist: {},
      },
    },
    recent_failures: [],
    ...overrides,
  };
}

function renderWithRouter(ui: React.ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

describe("ObservabilityCycles", () => {
  it("renders loading state when stats are unsettled", () => {
    renderWithRouter(<ObservabilityCycles stats={undefined} loading={true} />);
    expect(screen.getByText("Loading cycle telemetry…")).toBeInTheDocument();
  });

  it("renders unavailable state when stats settled to null", () => {
    renderWithRouter(<ObservabilityCycles stats={null} loading={false} />);
    expect(screen.getByText("Cycle telemetry unavailable.")).toBeInTheDocument();
  });

  it("renders the empty heatmap, empty bar, and friendly failures note on a fresh DB", () => {
    renderWithRouter(<ObservabilityCycles stats={emptyStats()} loading={false} />);
    expect(
      screen.getByText(
        /No execution cycles recorded yet — start a task to populate this view\./,
      ),
    ).toBeInTheDocument();
    // 4 phases × 4 statuses = 16 data cells, all rendered with "0".
    for (const phase of ["diagnose", "execute", "verify", "persist"] as const) {
      for (const status of ["running", "succeeded", "failed", "skipped"] as const) {
        const cell = screen.getByTestId(`obs-heatmap-cell-${phase}-${status}`);
        expect(cell).toHaveTextContent("0");
      }
    }
    expect(
      screen.getByText("No recent cycle failures — the agent worker is clean."),
    ).toBeInTheDocument();
  });

  it("renders cycle counts, heatmap data, and recent failures table", () => {
    const stats = emptyStats({
      cycles: {
        by_status: { running: 1, succeeded: 4, failed: 2, aborted: 1 },
        by_triggered_by: { agent: 7, user: 1 },
      },
      phases: {
        by_phase_status: {
          diagnose: { succeeded: 6 },
          execute: { succeeded: 4, failed: 2 },
          verify: { succeeded: 3, skipped: 1 },
          persist: { succeeded: 3 },
        },
      },
      recent_failures: [
        {
          task_id: "11111111-aaaa-bbbb-cccc-dddddddddddd",
          event_seq: 42,
          at: "2026-04-19T12:34:56Z",
          cycle_id: "cyc-1",
          attempt_seq: 2,
          status: "failed",
          reason: "execute blew up",
        },
        {
          task_id: "22222222-aaaa-bbbb-cccc-dddddddddddd",
          event_seq: 9,
          at: "2026-04-19T12:30:00Z",
          cycle_id: "cyc-2",
          attempt_seq: 1,
          status: "aborted",
          reason: "",
        },
      ],
    });
    renderWithRouter(<ObservabilityCycles stats={stats} loading={false} />);

    expect(screen.getByText("8 cycle attempts across all tasks.")).toBeInTheDocument();

    const exFailed = screen.getByTestId("obs-heatmap-cell-execute-failed");
    expect(exFailed).toHaveTextContent("2");
    expect(exFailed).toHaveAttribute("title", "Execute Failed: 2");
    const dxSucceeded = screen.getByTestId("obs-heatmap-cell-diagnose-succeeded");
    expect(dxSucceeded).toHaveTextContent("6");

    expect(
      screen.getByText("Last 2 cycle failures (newest first)"),
    ).toBeInTheDocument();
    const failuresSection = screen.getByLabelText("Recent cycle failures");
    const failureTable = within(failuresSection).getByRole("table");
    const rows = within(failureTable).getAllByRole("row");
    // header + 2 data rows
    expect(rows).toHaveLength(3);

    const firstRow = screen.getByTestId(
      "obs-failure-row-11111111-aaaa-bbbb-cccc-dddddddddddd-42",
    );
    const eventLink = within(firstRow).getByRole("link", {
      name: /Apr|2026|:/,
    });
    expect(eventLink).toHaveAttribute(
      "href",
      "/tasks/11111111-aaaa-bbbb-cccc-dddddddddddd/events/42",
    );
    expect(within(firstRow).getByText("execute blew up")).toBeInTheDocument();

    const abortedRow = screen.getByTestId(
      "obs-failure-row-22222222-aaaa-bbbb-cccc-dddddddddddd-9",
    );
    expect(within(abortedRow).getByText("aborted")).toBeInTheDocument();
    expect(
      within(abortedRow).getByText("(no reason recorded)"),
    ).toBeInTheDocument();
  });
});
