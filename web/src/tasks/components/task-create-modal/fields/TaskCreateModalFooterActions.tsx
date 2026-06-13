import type { PriorityChoice } from "@/types";
import { nonEmptyChecklistCount } from "@/tasks/task-compose/checklistRequirement";

type Props = {
  disabled: boolean;
  draftSaving: boolean;
  title: string;
  priority: PriorityChoice;
  checklistItems: string[];
  evaluatePending: boolean;
  onClose: () => void;
  onSaveDraft: () => void;
  onEvaluate: () => void;
};

export function TaskCreateModalFooterActions({
  disabled,
  draftSaving,
  title,
  priority,
  checklistItems,
  evaluatePending,
  onClose,
  onSaveDraft,
  onEvaluate,
}: Props) {
  const evalAndSubmitDisabled =
    !title.trim() ||
    !priority ||
    nonEmptyChecklistCount(checklistItems) < 1 ||
    disabled;

  const criteriaMet = nonEmptyChecklistCount(checklistItems) >= 1;

  return (
    <footer className="task-create-modal-actions">
      <button
        type="button"
        className="task-create-cancel-btn"
        disabled={disabled}
        onClick={onClose}
      >
        Cancel
      </button>
      <div className="task-create-modal-actions__end">
        <button
          type="button"
          className="task-create-save-draft-btn"
          disabled={disabled || draftSaving}
          onClick={onSaveDraft}
        >
          {draftSaving ? "Saving draft…" : "Save draft"}
        </button>
        <button
          type="button"
          className="task-create-evaluate-btn"
          disabled={evalAndSubmitDisabled}
          data-criteria-met={criteriaMet ? "true" : "false"}
          onClick={onEvaluate}
        >
          {evaluatePending ? "Evaluating…" : "Evaluate"}
        </button>
        <button
          type="submit"
          className="task-create-submit"
          disabled={evalAndSubmitDisabled}
        >
          Create task
        </button>
      </div>
    </footer>
  );
}
