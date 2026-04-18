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
        saving={false}
        deletePending={false}
        onCancel={onCancel}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByRole("dialog", {
        description: /this cannot be undone/i,
      }),
    ).toBeInTheDocument();
    await user.click(screen.getByRole("button", { name: /^cancel$/i }));
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel on Escape", async () => {
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(
      <DeleteConfirmDialog
        taskTitle="X"
        saving={false}
        deletePending={false}
        onCancel={onCancel}
        onConfirm={vi.fn()}
      />,
    );
    await user.keyboard("{Escape}");
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("calls onCancel on Escape while the delete is pending (dismissibleWhileBusy)", async () => {
    // Regression for the trap-the-user-behind-a-spinner papercut:
    // the underlying delete flow is id-aware (useTaskDeleteFlow),
    // so closing the dialog mid-flight is safe. The `Modal busy`
    // overlay must NOT swallow Escape here.
    const user = userEvent.setup();
    const onCancel = vi.fn();
    render(
      <DeleteConfirmDialog
        taskTitle="X"
        saving={false}
        deletePending
        onCancel={onCancel}
        onConfirm={vi.fn()}
      />,
    );
    await user.keyboard("{Escape}");
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("still renders the busy spinner overlay while pending", async () => {
    // Visual feedback must remain even when dismiss is allowed —
    // the user has to know the operation is still in flight.
    render(
      <DeleteConfirmDialog
        taskTitle="X"
        saving
        deletePending
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(screen.getByRole("status")).toBeInTheDocument();
  });
});
