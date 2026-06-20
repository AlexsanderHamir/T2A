import type { ChecklistItemDraft, PriorityChoice } from "@/types";
import { nonEmptyChecklistCount } from "@/tasks/task-compose/checklistRequirement";

type Props = {
  disabled: boolean;
  draftSaving: boolean;
  title: string;
  priority: PriorityChoice;
  checklistItems: ChecklistItemDraft[];
  onClose: () => void;
  onSaveDraft: () => void;
};

export function TaskCreateModalFooterActions({
  disabled,
  draftSaving,
  title,
  priority,
  checklistItems,
  onClose,
  onSaveDraft,
}: Props) {
  const submitDisabled =
    !title.trim() ||
    !priority ||
    nonEmptyChecklistCount(checklistItems) < 1 ||
    disabled;

  return (
    <div className="task-create-modal-actions">
      <div className="task-create-modal-actions__start">
        <button
          type="button"
          className="secondary task-create-cancel-btn"
          disabled={disabled}
          onClick={onClose}
        >
          Cancel
        </button>
      </div>
      <div className="task-create-modal-actions__end">
        <button
          type="button"
          className="secondary"
          disabled={disabled || draftSaving}
          onClick={onSaveDraft}
        >
          {draftSaving ? "Saving draft…" : "Save draft"}
        </button>
        <button
          type="submit"
          className="task-create-submit"
          disabled={submitDisabled}
        >
          Create task
        </button>
      </div>
    </div>
  );
}
