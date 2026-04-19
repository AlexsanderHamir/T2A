import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { TaskEditForm } from "./TaskEditForm";

function renderForm(props?: Partial<ComponentProps<typeof TaskEditForm>>) {
  const base: ComponentProps<typeof TaskEditForm> = {
    taskId: "task-123",
    title: "Existing title",
    prompt: "Existing prompt",
    priority: "medium",
    taskType: "general",
    status: "ready",
    checklistInherit: false,
    canInheritChecklist: true,
    saving: false,
    patchPending: false,
    onTitleChange: vi.fn(),
    onPromptChange: vi.fn(),
    onPriorityChange: vi.fn(),
    onTaskTypeChange: vi.fn(),
    onStatusChange: vi.fn(),
    onChecklistInheritChange: vi.fn(),
    onSubmit: vi.fn(),
    onCancel: vi.fn(),
  };
  return render(<TaskEditForm {...base} {...props} />);
}

describe("TaskEditForm", () => {
  it("renders the edit dialog with the task id and current title", () => {
    renderForm();
    expect(
      screen.getByRole("dialog", { name: /edit task/i }),
    ).toBeInTheDocument();
    expect(screen.getByDisplayValue(/existing title/i)).toBeInTheDocument();
  });

  it("calls onCancel on Escape while the patch is pending (dismissibleWhileBusy)", async () => {
    // Regression for the trap-the-user-behind-a-spinner papercut:
    // the underlying patch flow is id-aware (useTaskPatchFlow), so
    // closing the form mid-flight is safe. The `Modal busy` overlay
    // must NOT swallow Escape here.
    const user = userEvent.setup();
    const onCancel = vi.fn();
    renderForm({ patchPending: true, saving: true, onCancel });
    await user.keyboard("{Escape}");
    expect(onCancel).toHaveBeenCalledTimes(1);
  });

  it("still renders the busy spinner overlay while pending", () => {
    // Visual feedback must remain even when dismiss is allowed —
    // the user has to know the operation is still in flight.
    renderForm({ patchPending: true, saving: true });
    expect(screen.getByRole("status")).toBeInTheDocument();
  });

  describe("error prop (session #34: surface patchError inline)", () => {
    it("does not render an alert region when error is null", () => {
      // No `error` prop = no empty `role="alert"` callout in the DOM.
      // Pins the "no live-region churn at idle" contract.
      renderForm();
      expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    });

    it("renders the underlying patch error message when error is set", () => {
      // Pins the user-visible feedback path: when the global ErrorBanner
      // is hidden behind the modal backdrop, the user must still see
      // why the PATCH failed (#33-style gap closed for the edit flow).
      renderForm({ error: "title cannot be empty" });
      const alert = screen.getByRole("alert");
      expect(alert).toHaveTextContent(/title cannot be empty/i);
    });

    it("keeps action buttons enabled when an error is showing so the user can retry", () => {
      // Same retry-affordance contract as the create / evaluate / subtask
      // / checklist / delete callouts (#31-#34): the user must be able
      // to click Save again immediately. `saving` is false here because
      // the mutation has settled into an error.
      renderForm({ error: "boom" });
      expect(
        screen.getByRole("button", { name: /^save$/i }),
      ).not.toBeDisabled();
      expect(
        screen.getByRole("button", { name: /^cancel$/i }),
      ).not.toBeDisabled();
    });
  });
});
