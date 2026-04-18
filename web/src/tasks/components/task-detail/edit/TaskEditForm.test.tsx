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
});
