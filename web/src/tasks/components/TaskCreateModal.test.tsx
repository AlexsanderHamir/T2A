import { render, screen } from "@testing-library/react";
import { within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { TaskCreateModal } from "./TaskCreateModal";

function renderModal(props?: Partial<ComponentProps<typeof TaskCreateModal>>) {
  const base: ComponentProps<typeof TaskCreateModal> = {
    pending: false,
    saving: false,
    draftSaving: false,
    draftSaveLabel: null,
    draftSaveError: false,
    onClose: vi.fn(),
    title: "Draft title",
    prompt: "Draft prompt",
    priority: "medium",
    taskType: "general",
    checklistItems: [],
    parentOptions: [],
    parentId: "",
    checklistInherit: false,
    onTitleChange: vi.fn(),
    onPromptChange: vi.fn(),
    onPriorityChange: vi.fn(),
    onTaskTypeChange: vi.fn(),
    onParentIdChange: vi.fn(),
    onChecklistInheritChange: vi.fn(),
    onAppendChecklistCriterion: vi.fn(),
    onUpdateChecklistRow: vi.fn(),
    onRemoveChecklistRow: vi.fn(),
    pendingSubtasks: [],
    onAddPendingSubtask: vi.fn(),
    onUpdatePendingSubtask: vi.fn(),
    onRemovePendingSubtask: vi.fn(),
    evaluatePending: false,
    evaluation: null,
    draftName: "Untitled draft",
    onDraftNameChange: vi.fn(),
    onSaveDraft: vi.fn(),
    onEvaluate: vi.fn(),
    onSubmit: vi.fn(),
  };
  return render(<TaskCreateModal {...base} {...props} />);
}

describe("TaskCreateModal", () => {
  it("shows Evaluate action and calls onEvaluate", async () => {
    const user = userEvent.setup();
    const onEvaluate = vi.fn();
    renderModal({ onEvaluate });
    await user.click(screen.getByRole("button", { name: /^evaluate$/i }));
    expect(onEvaluate).toHaveBeenCalledTimes(1);
  });

  it("renders evaluation summary when available", () => {
    renderModal({
      evaluation: {
        overallScore: 86,
        overallSummary: "Strong draft, likely ready for creation.",
        sections: [
          { key: "title", score: 90 },
          { key: "initial_prompt", score: 84 },
        ],
      },
    });
    const panel = screen.getByRole("region", {
      name: /draft evaluation summary/i,
    });
    expect(
      within(panel).getByRole("heading", { name: /latest evaluation score/i }),
    ).toBeInTheDocument();
    expect(within(panel).getByText(/86/i)).toBeInTheDocument();
    expect(within(panel).getByText(/title/i)).toBeInTheDocument();
  });

  it("shows where score appears before evaluation", () => {
    renderModal({ evaluation: null });
    const panel = screen.getByRole("region", {
      name: /draft evaluation summary/i,
    });
    expect(within(panel).getByText(/no score yet/i)).toBeInTheDocument();
    expect(within(panel).getByText(/click/i)).toBeInTheDocument();
  });

  it("shows Save draft action and calls onSaveDraft", async () => {
    const user = userEvent.setup();
    const onSaveDraft = vi.fn();
    renderModal({ onSaveDraft });
    await user.click(screen.getByRole("button", { name: /save draft/i }));
    expect(onSaveDraft).toHaveBeenCalledTimes(1);
  });

  it("disables Save draft while draft save is pending", () => {
    renderModal({ draftSaving: true });
    expect(
      screen.getByRole("button", { name: /saving draft/i }),
    ).toBeDisabled();
  });

  it("renders parent options loading skeleton while parent options are pending", () => {
    renderModal({ parentOptionsLoading: true });
    expect(
      document.querySelector(".task-create-parent-loading"),
    ).not.toBeNull();
    expect(screen.getByRole("status")).toHaveTextContent(
      /loading parent task options/i,
    );
  });
});
