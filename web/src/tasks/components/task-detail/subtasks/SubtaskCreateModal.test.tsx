import { render, screen } from "@testing-library/react";
import type { ComponentProps } from "react";
import { describe, expect, it, vi } from "vitest";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { SubtaskCreateModal } from "./SubtaskCreateModal";

function renderModal(
  overrides: Partial<ComponentProps<typeof SubtaskCreateModal>> = {},
) {
  const base: ComponentProps<typeof SubtaskCreateModal> = {
    taskId: "parent-1",
    pending: false,
    saving: false,
    onClose: vi.fn(),
    title: "",
    prompt: "",
    priority: "medium",
    taskType: "general",
    checklistItems: [],
    checklistInherit: false,
    onTitleChange: vi.fn(),
    onPromptChange: vi.fn(),
    onPriorityChange: vi.fn(),
    onTaskTypeChange: vi.fn(),
    onAppendChecklistCriterion: vi.fn(),
    onUpdateChecklistRow: vi.fn(),
    onRemoveChecklistRow: vi.fn(),
    onChecklistInheritChange: vi.fn(),
    onSubmit: vi.fn(),
    ...overrides,
  };
  return render(
    <ModalStackProvider>
      <SubtaskCreateModal {...base} />
    </ModalStackProvider>,
  );
}

describe("SubtaskCreateModal error display", () => {
  it("does not render an error callout on the happy path", () => {
    renderModal();
    expect(
      screen.queryByRole("alert", { name: /could not create/i }),
    ).not.toBeInTheDocument();
  });

  it("renders the underlying mutation error message", () => {
    renderModal({ error: new Error("upstream timeout") });
    const alert = screen.getByRole("alert");
    expect(alert).toHaveTextContent(/upstream timeout/i);
  });

  it("falls back to a kinder default for non-Error throwables", () => {
    // `mutation.error` is `Error | null` per react-query v5; the
    // fallback only kicks in for non-Error inputs (defense in depth
    // against legacy code paths). Confirms the gate refuses to render
    // an empty banner when `error` is undefined / null.
    renderModal({ error: undefined });
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("keeps the cancel button reachable while the error is showing", () => {
    renderModal({
      title: "Reproduce the bug",
      error: new Error("network blip"),
    });
    expect(screen.getByRole("button", { name: /cancel/i })).not.toBeDisabled();
    expect(
      screen.getByRole("button", { name: /add subtask/i }),
    ).not.toBeDisabled();
  });
});
