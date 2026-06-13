import type { PriorityChoice } from "@/types";

type Props = {
  disabled: boolean;
  draftSaving: boolean;
  title: string;
  priority: PriorityChoice;
  dmapReady: boolean;
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
  dmapReady,
  evaluatePending,
  onClose,
  onSaveDraft,
  onEvaluate,
}: Props) {
  const evalAndSubmitDisabled = !title.trim() || !priority || !dmapReady || disabled;

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
          type="button"
          className="secondary task-create-evaluate-btn"
          disabled={evalAndSubmitDisabled}
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
    </div>
  );
}
