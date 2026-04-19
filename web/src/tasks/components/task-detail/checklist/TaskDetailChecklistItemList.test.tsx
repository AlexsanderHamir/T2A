import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { TaskChecklistItemView } from "@/types";
import { TaskDetailChecklistItemList } from "./TaskDetailChecklistItemList";

const PENDING: TaskChecklistItemView = {
  id: "pending-1",
  sort_order: 1,
  text: "Hello World is written inside the 123.md file",
  done: false,
};

const DONE: TaskChecklistItemView = {
  id: "done-1",
  sort_order: 2,
  text: "123.md file created",
  done: true,
};

function renderList(items: TaskChecklistItemView[], overrides?: Partial<{
  editCriterionPending: boolean;
  removeItemPending: boolean;
  addCriterionPending: boolean;
}>) {
  const onOpenEditCriterionModal = vi.fn();
  const onRemoveChecklistItem = vi.fn();
  render(
    <TaskDetailChecklistItemList
      items={items}
      checklistInherit={false}
      editCriterionPending={overrides?.editCriterionPending ?? false}
      removeItemPending={overrides?.removeItemPending ?? false}
      addCriterionPending={overrides?.addCriterionPending ?? false}
      onOpenEditCriterionModal={onOpenEditCriterionModal}
      onRemoveChecklistItem={onRemoveChecklistItem}
    />,
  );
  return { onOpenEditCriterionModal, onRemoveChecklistItem };
}

describe("TaskDetailChecklistItemList", () => {
  // Pins the rule from the user-reported bug: once the agent has
  // marked a criterion done, the Edit affordance must lock — editing
  // the definition text after acceptance silently rewrites the
  // already-emitted checklist_item_toggled audit row's meaning. The
  // backend rejects this with ErrInvalidInput; this test guards the
  // UI half so users can't even attempt the bogus write.
  it("locks the Edit button on done criteria and explains why", () => {
    renderList([PENDING, DONE]);

    const editButtons = screen.getAllByRole("button", { name: /^edit/i });
    expect(editButtons).toHaveLength(2);

    // Pending row: editable.
    const editPending = editButtons.find((b) => !b.hasAttribute("disabled"));
    expect(editPending).toBeDefined();
    expect(editPending).not.toBeDisabled();

    // Done row: disabled with a clear explanatory tooltip.
    const editDone = editButtons.find((b) =>
      b.getAttribute("aria-label")?.includes("locked"),
    );
    expect(editDone).toBeDefined();
    expect(editDone).toBeDisabled();
    expect(editDone).toHaveAttribute(
      "title",
      expect.stringMatching(/already marked done/i),
    );
  });

  it("does not call onOpenEditCriterionModal when the locked Edit button is clicked", async () => {
    const user = userEvent.setup();
    const { onOpenEditCriterionModal } = renderList([DONE]);

    const editDone = screen.getByRole("button", { name: /edit.*locked/i });
    // userEvent honors `disabled` and skips the click; we still
    // explicitly assert no handler invocation, since that is the
    // contract this test pins.
    await user.click(editDone);
    expect(onOpenEditCriterionModal).not.toHaveBeenCalled();
  });

  it("keeps Remove enabled on done criteria so users can clean up wrong acceptances", () => {
    renderList([DONE]);

    // Remove must remain available even when Edit is locked: a done
    // row that was wrongly auto-completed still needs an escape
    // hatch, otherwise the criterion is permanent dead weight.
    const removeBtn = screen.getByRole("button", { name: /^remove$/i });
    expect(removeBtn).toBeEnabled();
  });
});
