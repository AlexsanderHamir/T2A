import type { ChecklistItemDraft, PriorityChoice } from "@/types";
import { nonEmptyChecklistCount } from "@/tasks/task-compose/checklistRequirement";

type Props = {
  variant: "task-create" | "template";
  disabled: boolean;
  draftSaving: boolean;
  title: string;
  priority: PriorityChoice;
  checklistItems: ChecklistItemDraft[];
  worktreeId?: string;
  branchId?: string;
  worktreeBranchId?: string;
  requireGitBinding?: boolean;
  onClose: () => void;
  onSaveDraft?: () => void;
};

export function TaskCreateModalFooterActions({
  variant,
  disabled,
  draftSaving,
  title,
  priority,
  checklistItems,
  worktreeId = "",
  branchId = "",
  worktreeBranchId = "",
  requireGitBinding = false,
  onClose,
  onSaveDraft,
}: Props) {
  const gitBindingIncomplete =
    requireGitBinding &&
    worktreeBranchId.trim() === "" &&
    (!worktreeId.trim() || !branchId.trim());
  const submitDisabled =
    !title.trim() ||
    !priority ||
    nonEmptyChecklistCount(checklistItems) < 1 ||
    gitBindingIncomplete ||
    disabled;

  const submitLabel = variant === "template" ? "Save template" : "Create task";

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
        {variant === "task-create" && onSaveDraft ? (
          <button
            type="button"
            className="secondary"
            disabled={disabled || draftSaving}
            onClick={onSaveDraft}
          >
            {draftSaving ? "Saving draft…" : "Save draft"}
          </button>
        ) : null}
        <button
          type="submit"
          className="task-create-submit"
          disabled={submitDisabled}
        >
          {submitLabel}
        </button>
      </div>
    </div>
  );
}
