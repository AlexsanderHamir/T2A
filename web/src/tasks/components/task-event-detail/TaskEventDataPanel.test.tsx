import { render, screen } from "@testing-library/react";
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
});
