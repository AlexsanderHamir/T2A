import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { AutonomyConfirmDialog } from "./AutonomyConfirmDialog";

describe("AutonomyConfirmDialog", () => {
  it("uses 'Resume' copy and Resume button when enable=true", () => {
    render(
      <AutonomyConfirmDialog
        enable
        taskTitle="My task"
        saving={false}
        pending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("dialog", { name: /resume autonomous execution/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^resume$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/agent will pick this task up/i),
    ).toBeInTheDocument();
  });

  it("uses 'Put on hold' copy and confirm button when enable=false", () => {
    render(
      <AutonomyConfirmDialog
        enable={false}
        taskTitle="My task"
        saving={false}
        pending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("dialog", { name: /put this task on hold/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: /^put on hold$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(/agent will stop considering this task/i),
    ).toBeInTheDocument();
  });

  it("calls onConfirm when the confirm button is clicked", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <AutonomyConfirmDialog
        enable
        taskTitle="t"
        saving={false}
        pending={false}
        onCancel={vi.fn()}
        onConfirm={onConfirm}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^resume$/i }));
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel from the Cancel button and Escape", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(
      <AutonomyConfirmDialog
        enable={false}
        taskTitle="t"
        saving={false}
        pending={false}
        onCancel={onCancel}
        onConfirm={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));
    await user.keyboard("{Escape}");
    expect(onCancel).toHaveBeenCalledTimes(2);
  });

  it("renders inline error and keeps buttons enabled for retry", () => {
    render(
      <AutonomyConfirmDialog
        enable
        taskTitle="t"
        saving={false}
        pending={false}
        error="couldn't update"
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(screen.getByRole("alert")).toHaveTextContent(/couldn't update/i);
    expect(
      screen.getByRole("button", { name: /^resume$/i }),
    ).not.toBeDisabled();
  });
});
