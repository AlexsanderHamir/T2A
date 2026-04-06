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
    expect(within(panel).getByText(/86/)).toBeInTheDocument();
    expect(within(panel).getByText(/title/i)).toBeInTheDocument();
  });
});
