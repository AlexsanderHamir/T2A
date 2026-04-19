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
        description: /will be permanently deleted/i,
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

  it("does not render the cascade callout when subtaskCount is 0/omitted", () => {
    render(
      <DeleteConfirmDialog
        taskTitle="leaf"
        saving={false}
        deletePending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(screen.queryByRole("note")).not.toBeInTheDocument();
    expect(
      screen.queryByText(/subtask.*will also be deleted/i),
    ).not.toBeInTheDocument();
  });

  it("renders the cascade callout when subtaskCount > 0 (singular phrasing)", () => {
    render(
      <DeleteConfirmDialog
        taskTitle="parent"
        subtaskCount={1}
        saving={false}
        deletePending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    const callout = screen.getByRole("note");
    expect(callout).toBeInTheDocument();
    expect(callout).toHaveTextContent(/1 subtask will also be deleted\./i);
  });

  it("renders the cascade callout when subtaskCount > 1 (plural phrasing)", () => {
    render(
      <DeleteConfirmDialog
        taskTitle="parent"
        subtaskCount={4}
        saving={false}
        deletePending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    const callout = screen.getByRole("note");
    expect(callout).toBeInTheDocument();
    expect(callout).toHaveTextContent(/4 subtasks will also be deleted\./i);
  });

  it("always renders the muted 'cannot be undone' footnote", () => {
    render(
      <DeleteConfirmDialog
        taskTitle="X"
        saving={false}
        deletePending={false}
        onCancel={vi.fn()}
        onConfirm={vi.fn()}
      />,
    );
    expect(
      screen.getByText(/this action cannot be undone\./i),
    ).toBeInTheDocument();
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

  describe("error prop (session #34: surface deleteError inline)", () => {
    it("does not render an alert region when error is null", () => {
      // No `error` prop = no empty `role="alert"` callout in the DOM.
      // Pins the "no live-region churn at idle" contract.
      render(
        <DeleteConfirmDialog
          taskTitle="X"
          saving={false}
          deletePending={false}
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("renders the underlying delete error message when error is set", () => {
      // Pins the user-visible feedback path: when the global ErrorBanner
      // is hidden behind the modal backdrop, the user must still see
      // why the DELETE failed (#33-style gap closed for the delete flow).
      render(
        <DeleteConfirmDialog
          taskTitle="X"
          saving={false}
          deletePending={false}
          error="task busy: cannot delete"
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/task busy: cannot delete/i);
    });

    it("keeps action buttons enabled when an error is showing so the user can retry", () => {
      // Same retry-affordance contract as the create / evaluate / subtask
      // / checklist callouts (#31-#33): the user must be able to click
      // Delete again immediately. `saving` is false here because the
      // mutation has settled into an error (react-query flips
      // `isPending` back to `false` on settle).
      render(
        <DeleteConfirmDialog
          taskTitle="X"
          saving={false}
          deletePending={false}
          error="boom"
          onCancel={vi.fn()}
          onConfirm={vi.fn()}
        />,
      );
      expect(
        screen.getByRole("button", { name: /^cancel$/i }),
      ).not.toBeDisabled();
      expect(
        screen.getByRole("button", { name: /^delete$/i }),
      ).not.toBeDisabled();
    });
  });
});
