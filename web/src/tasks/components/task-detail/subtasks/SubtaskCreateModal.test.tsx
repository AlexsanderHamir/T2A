import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ComponentProps, FormEvent, ReactNode } from "react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { SubtaskCreateModal } from "./SubtaskCreateModal";

function renderModal(
  overrides: Partial<ComponentProps<typeof SubtaskCreateModal>> = {},
) {
  const onSubmit = vi.fn((e: FormEvent) => e.preventDefault());
  const props: ComponentProps<typeof SubtaskCreateModal> = {
    taskId: "parent-1",
    pending: false,
    saving: false,
    onClose: vi.fn(),
    title: "New sub",
    prompt: "",
    priority: "medium",
    taskType: "general",
    checklistItems: [],
    checklistInherit: false,
    waitForParent: false,
    onWaitForParentChange: vi.fn(),
    siblingOptions: [
      { id: "sib-1", label: "First subtask" },
      { id: "sib-2", label: "Second subtask" },
    ],
    dependsOnSiblingIds: [],
    onDependsOnSiblingIdsChange: vi.fn(),
    onTitleChange: vi.fn(),
    onPromptChange: vi.fn(),
    onPriorityChange: vi.fn(),
    onTaskTypeChange: vi.fn(),
    onAppendChecklistCriterion: vi.fn(),
    onUpdateChecklistRow: vi.fn(),
    onRemoveChecklistRow: vi.fn(),
    onChecklistInheritChange: vi.fn(),
    onSubmit,
    ...overrides,
  };

  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider
        client={
          new QueryClient({
            defaultOptions: { queries: { retry: false } },
          })
        }
      >
        {children}
      </QueryClientProvider>
    );
  }

  return {
    onSubmit,
    ...render(<SubtaskCreateModal {...props} />, { wrapper: Wrapper }),
  };
}

describe("SubtaskCreateModal scheduling", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("calls scheduling change handlers when parent and sibling boxes are toggled", async () => {
    const user = userEvent.setup();
    const onWaitForParentChange = vi.fn();
    const onDependsOnSiblingIdsChange = vi.fn();

    renderModal({ onWaitForParentChange, onDependsOnSiblingIdsChange });

    await user.click(
      screen.getByRole("checkbox", { name: /start after parent criteria pass/i }),
    );
    expect(onWaitForParentChange).toHaveBeenCalledWith(true);

    await user.click(
      screen.getByRole("checkbox", { name: /first subtask/i }),
    );
    expect(onDependsOnSiblingIdsChange).toHaveBeenCalledWith(["sib-1"]);
  });
});
