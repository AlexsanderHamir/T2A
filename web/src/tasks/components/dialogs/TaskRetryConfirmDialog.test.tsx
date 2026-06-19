import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TaskRetryConfirmDialog } from "./TaskRetryConfirmDialog";

describe("TaskRetryConfirmDialog", () => {
  it("uses Start over copy for fresh mode", () => {
    render(
      <TaskRetryConfirmDialog
        mode="fresh"
        taskTitle="My task"
        saving={false}
        pending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("heading", { name: /start over\?/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/discards this attempt's git changes/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^start over$/i }),
    ).toBeInTheDocument();
  });

  it("uses Resume from failure copy for resume mode", () => {
    render(
      <TaskRetryConfirmDialog
        mode="resume"
        taskTitle="My task"
        saving={false}
        pending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("heading", { name: /resume from failure\?/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/continues from the failed attempt's checkpoint/i),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^resume from failure$/i }),
    ).toBeInTheDocument();
  });

  it("invokes onConfirm when the confirm button is clicked", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <TaskRetryConfirmDialog
        mode="fresh"
        taskTitle="My task"
        saving={false}
        pending={false}
        onCancel={vi.fn()}
        onConfirm={onConfirm}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^start over$/i }));
    expect(onConfirm).toHaveBeenCalledOnce();
  });
});
