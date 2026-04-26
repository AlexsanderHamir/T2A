import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { TaskEventDataPanel } from "./TaskEventDataPanel";

describe("TaskEventDataPanel", () => {
  it("renders cycle_failed overview with failure summary and reason code", () => {
    render(
      <TaskEventDataPanel
        eventType="cycle_failed"
        data={{
          cycle_id: "c1",
          attempt_seq: 1,
          status: "failed",
          reason: "runner_non_zero_exit",
          failure_summary: "Operator-visible failure text.",
        }}
      />,
    );
    expect(
      screen.getByText("Operator-visible failure text."),
    ).toBeInTheDocument();
    expect(screen.getByText("runner_non_zero_exit")).toBeInTheDocument();
  });

  it("renders GFM markdown tables in phase summary", () => {
    render(
      <TaskEventDataPanel
        eventType="phase_completed"
        data={{
          phase: "execute",
          status: "succeeded",
          summary: "| File | Content |\n| --- | --- |\n| 1.md | hello 1 |",
        }}
      />,
    );
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "File" })).toBeInTheDocument();
    expect(screen.getByRole("cell", { name: "1.md" })).toBeInTheDocument();
  });

  it("renders tables when summary uses escaped newlines", () => {
    render(
      <TaskEventDataPanel
        eventType="phase_completed"
        data={{
          phase: "execute",
          status: "succeeded",
          summary: "| A | B |\\n| --- | --- |\\n| x | y |",
        }}
      />,
    );
    expect(screen.getByRole("table")).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "A" })).toBeInTheDocument();
  });

  it("moves tab selection with arrow, home, and end keys", async () => {
    const user = userEvent.setup();
    render(
      <TaskEventDataPanel
        eventType="task_created"
        data={{
          task_id: "t1",
          title: "Task",
        }}
      />,
    );

    const overviewTab = screen.getByRole("tab", { name: "Overview" });
    const jsonTab = screen.getByRole("tab", { name: "Raw JSON" });

    overviewTab.focus();
    expect(overviewTab).toHaveFocus();

    await user.keyboard("{ArrowRight}");
    expect(jsonTab).toHaveAttribute("aria-selected", "true");
    expect(jsonTab).toHaveFocus();

    await user.keyboard("{Home}");
    expect(overviewTab).toHaveAttribute("aria-selected", "true");
    expect(overviewTab).toHaveFocus();

    await user.keyboard("{End}");
    expect(jsonTab).toHaveAttribute("aria-selected", "true");
    expect(jsonTab).toHaveFocus();

    await user.keyboard("{ArrowLeft}");
    expect(overviewTab).toHaveAttribute("aria-selected", "true");
    expect(overviewTab).toHaveFocus();
  });
});
