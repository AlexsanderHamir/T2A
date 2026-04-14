import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../../../../lib/routerFutureFlags";
import { TaskDetailSubtasksHead } from "./TaskDetailSubtasksHead";

describe("TaskDetailSubtasksHead", () => {
  it("renders graph link and add button", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailSubtasksHead
          taskId="abc-1"
          saving={false}
          onAddSubtask={vi.fn()}
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: /^subtasks$/i })).toHaveAttribute(
      "id",
      "task-subtasks-heading",
    );
    expect(screen.getByRole("link", { name: /open graph view/i })).toHaveAttribute(
      "href",
      "/tasks/abc-1/graph",
    );
    expect(screen.getByRole("button", { name: /add subtask/i })).toBeEnabled();
  });

  it("disables add subtask while saving", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailSubtasksHead taskId="t1" saving onAddSubtask={vi.fn()} />
      </MemoryRouter>,
    );

    expect(screen.getByRole("button", { name: /add subtask/i })).toBeDisabled();
  });

  it("calls onAddSubtask when add is clicked", async () => {
    const user = userEvent.setup();
    const onAddSubtask = vi.fn();
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailSubtasksHead
          taskId="t1"
          saving={false}
          onAddSubtask={onAddSubtask}
        />
      </MemoryRouter>,
    );

    await user.click(screen.getByRole("button", { name: /add subtask/i }));
    expect(onAddSubtask).toHaveBeenCalledOnce();
  });
});
