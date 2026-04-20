import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../../../../lib/routerFutureFlags";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { TaskDetailHeader } from "./TaskDetailHeader";

describe("TaskDetailHeader", () => {
  it("renders title, stance, status and priority pills, and back link", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailHeader
          task={{
            title: "My task",
            status: "ready",
            priority: "high",
            ...TASK_TEST_DEFAULTS,
          }}
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: /^my task$/i })).toBeInTheDocument();
    expect(screen.getByText("Informational")).toHaveAttribute(
      "data-stance",
      "informational",
    );
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.getByText("high")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /← all tasks/i })).toHaveAttribute(
      "href",
      "/",
    );
  });

  it("marks stance when status needs user input", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailHeader
          task={{
            title: "Blocked",
            status: "blocked",
            priority: "medium",
            ...TASK_TEST_DEFAULTS,
            cursor_model: "opus",
          }}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText("Agent needs input")).toHaveAttribute(
      "data-stance",
      "needs-user",
    );
    expect(screen.getByText("blocked")).toHaveAttribute("data-needs-user", "true");
  });

  it("renders the runtime chip with runner and model intent (Phase 4a of plan)", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailHeader
          task={{
            title: "Has model",
            status: "ready",
            priority: "medium",
            ...TASK_TEST_DEFAULTS,
            cursor_model: "opus-4",
          }}
        />
      </MemoryRouter>,
    );
    const chip = screen.getByTestId("task-detail-runtime");
    expect(chip).toHaveTextContent("Cursor CLI · opus-4");
    expect(chip.className).toContain("cell-pill--runtime");
  });

  it("renders 'default model' copy in the runtime chip when task has no cursor_model selected", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailHeader
          task={{
            title: "No model",
            status: "ready",
            priority: "medium",
            ...TASK_TEST_DEFAULTS,
            cursor_model: "",
          }}
        />
      </MemoryRouter>,
    );
    expect(screen.getByTestId("task-detail-runtime")).toHaveTextContent(
      "Cursor CLI · default model",
    );
  });
});
