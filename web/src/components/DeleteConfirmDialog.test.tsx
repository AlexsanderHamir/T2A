import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { DeleteConfirmDialog } from "./DeleteConfirmDialog";

describe("DeleteConfirmDialog", () => {
  it("calls onCancel when Cancel is clicked", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(
      <DeleteConfirmDialog
        taskTitle="My task"
        busy={false}
        onCancel={onCancel}
        onConfirm={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel on Escape", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(
      <DeleteConfirmDialog
        taskTitle="X"
        busy={false}
        onCancel={onCancel}
        onConfirm={vi.fn()}
      />,
    );
    await user.keyboard("{Escape}");
    expect(onCancel).toHaveBeenCalledTimes(1);
  });
});
