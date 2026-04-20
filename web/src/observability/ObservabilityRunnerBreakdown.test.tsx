import { render, screen, within } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type {
  TaskStatsResponse,
  TaskStatsRunnerBucket,
} from "@/types/task";
import { ObservabilityRunnerBreakdown } from "./ObservabilityRunnerBreakdown";

function bucket(
  overrides: Partial<TaskStatsRunnerBucket> = {},
): TaskStatsRunnerBucket {
  return {
    by_status: {},
    succeeded: 0,
    duration_p50_succeeded_seconds: 0,
    duration_p95_succeeded_seconds: 0,
    ...overrides,
  };
}

function statsWithRunner(runner: TaskStatsResponse["runner"]): TaskStatsResponse {
  return {
    total: 0,
    ready: 0,
    critical: 0,
    scheduled: 0,
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
    runner,
    recent_failures: [],
  };
}

describe("ObservabilityRunnerBreakdown", () => {
  it("renders loading copy while stats are unsettled", () => {
    render(
      <ObservabilityRunnerBreakdown stats={undefined} loading={true} />,
    );
    expect(
      screen.getByText("Loading runner attribution…"),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("obs-runner-breakdown-table"),
    ).not.toBeInTheDocument();
  });

  it("renders unavailable copy when stats settled to null", () => {
    render(<ObservabilityRunnerBreakdown stats={null} loading={false} />);
    expect(
      screen.getByText("Runner attribution unavailable."),
    ).toBeInTheDocument();
  });

  it("renders the empty-state subtitle when no runner cycles have been recorded", () => {
    render(
      <ObservabilityRunnerBreakdown
        stats={statsWithRunner({
          by_runner: {},
          by_model: {},
          by_runner_model: {},
        })}
        loading={false}
      />,
    );
    expect(
      screen.getByText(
        /No runner attribution yet — start a task to populate this view\./,
      ),
    ).toBeInTheDocument();
    expect(
      screen.queryByTestId("obs-runner-breakdown-table"),
    ).not.toBeInTheDocument();
  });

  it("renders a single runner with a single model and computes success rate + percentiles", () => {
    const stats = statsWithRunner({
      by_runner: {
        cursor: bucket({
          by_status: { succeeded: 9, failed: 1 },
          succeeded: 9,
          duration_p50_succeeded_seconds: 12,
          duration_p95_succeeded_seconds: 34,
        }),
      },
      by_model: {
        "opus-4": bucket({
          by_status: { succeeded: 9, failed: 1 },
          succeeded: 9,
          duration_p50_succeeded_seconds: 12,
          duration_p95_succeeded_seconds: 34,
        }),
      },
      by_runner_model: {
        "cursor|opus-4": bucket({
          by_status: { succeeded: 9, failed: 1 },
          succeeded: 9,
          duration_p50_succeeded_seconds: 12,
          duration_p95_succeeded_seconds: 34,
        }),
      },
    });
    render(
      <ObservabilityRunnerBreakdown stats={stats} loading={false} />,
    );

    expect(
      screen.getByText("1 runner · 1 model · 10 total cycles"),
    ).toBeInTheDocument();

    const totalRow = screen.getByTestId(
      "obs-runner-row-runner|cursor|__total__",
    );
    expect(within(totalRow).getByText("Cursor CLI · all models")).toBeInTheDocument();
    // total = 10, succeeded = 9, failed = 1, aborted = 0, running = 0
    expect(within(totalRow).getByText("10")).toBeInTheDocument();
    expect(within(totalRow).getByText("9")).toBeInTheDocument();
    expect(within(totalRow).getByText("1")).toBeInTheDocument();
    // 9 / (9+1+0) = 90%
    expect(within(totalRow).getByText("90%")).toBeInTheDocument();
    expect(within(totalRow).getByText("12.0 s")).toBeInTheDocument();
    expect(within(totalRow).getByText("34.0 s")).toBeInTheDocument();

    const modelRow = screen.getByTestId(
      "obs-runner-row-runner|cursor|opus-4",
    );
    expect(within(modelRow).getByText("Cursor CLI · opus-4")).toBeInTheDocument();
  });

  it("orders per-runner rows by total cycles descending and nests per-model rows under each", () => {
    const stats = statsWithRunner({
      by_runner: {
        cursor: bucket({
          by_status: { succeeded: 3, failed: 1 },
          succeeded: 3,
        }),
        claude: bucket({
          by_status: { succeeded: 10, failed: 2, aborted: 1 },
          succeeded: 10,
        }),
      },
      by_model: {
        "opus-4": bucket({ by_status: { succeeded: 3 }, succeeded: 3 }),
        "sonnet-4.5": bucket({
          by_status: { succeeded: 10, failed: 2, aborted: 1 },
          succeeded: 10,
        }),
        "opus-4-claude": bucket({ by_status: { failed: 1 }, succeeded: 0 }),
      },
      by_runner_model: {
        "cursor|opus-4": bucket({
          by_status: { succeeded: 3 },
          succeeded: 3,
        }),
        "cursor|": bucket({ by_status: { failed: 1 }, succeeded: 0 }),
        "claude|sonnet-4.5": bucket({
          by_status: { succeeded: 10, failed: 2, aborted: 1 },
          succeeded: 10,
        }),
      },
    });
    render(
      <ObservabilityRunnerBreakdown stats={stats} loading={false} />,
    );

    const rows = screen.getAllByRole("row");
    // header + 2 runner-totals + 3 model rows = 6
    expect(rows.length).toBe(6);

    // claude has 13 cycles, cursor has 4 → claude must come first. `claude`
    // isn't in RUNNER_LABELS, so runnerLabel() falls through to the verbatim
    // adapter name; the label reads "claude · all models".
    const dataRows = rows.slice(1);
    expect(dataRows[0].textContent).toContain("claude · all models");
    expect(dataRows[1].textContent).toContain(
      "claude · sonnet-4.5",
    );
    expect(dataRows[2].textContent).toContain("Cursor CLI · all models");
    // Cursor's model rows: opus-4 (3 cycles) ranks above default model (1).
    expect(dataRows[3].textContent).toContain("Cursor CLI · opus-4");
    expect(dataRows[4].textContent).toContain("Cursor CLI · default model");
  });

  it("renders '—' in percentile cells when a bucket has no succeeded cycles", () => {
    const stats = statsWithRunner({
      by_runner: {
        cursor: bucket({
          by_status: { failed: 2, aborted: 1 },
          succeeded: 0,
        }),
      },
      by_model: {
        "opus-4": bucket({
          by_status: { failed: 2, aborted: 1 },
          succeeded: 0,
        }),
      },
      by_runner_model: {
        "cursor|opus-4": bucket({
          by_status: { failed: 2, aborted: 1 },
          succeeded: 0,
        }),
      },
    });
    render(
      <ObservabilityRunnerBreakdown stats={stats} loading={false} />,
    );
    const row = screen.getByTestId("obs-runner-row-runner|cursor|opus-4");
    const emDashes = within(row).getAllByText("—");
    // Two percentile cells render "—"; success rate renders a number (0%),
    // not "—", because terminal cycles exist (failed + aborted = 3).
    expect(emDashes.length).toBe(2);
    expect(within(row).getByText("0%")).toBeInTheDocument();
  });

  it("labels pre-feature model buckets as 'default model'", () => {
    const stats = statsWithRunner({
      by_runner: {
        cursor: bucket({
          by_status: { succeeded: 1 },
          succeeded: 1,
          duration_p50_succeeded_seconds: 5,
          duration_p95_succeeded_seconds: 5,
        }),
      },
      by_model: {
        "": bucket({
          by_status: { succeeded: 1 },
          succeeded: 1,
          duration_p50_succeeded_seconds: 5,
          duration_p95_succeeded_seconds: 5,
        }),
      },
      by_runner_model: {
        "cursor|": bucket({
          by_status: { succeeded: 1 },
          succeeded: 1,
          duration_p50_succeeded_seconds: 5,
          duration_p95_succeeded_seconds: 5,
        }),
      },
    });
    render(
      <ObservabilityRunnerBreakdown stats={stats} loading={false} />,
    );
    expect(
      screen.getByText("Cursor CLI · default model"),
    ).toBeInTheDocument();
  });
});
