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
    <div className="row stack-row-actions task-create-modal-actions">
      <button
        type="button"
        className="secondary task-create-cancel-btn"
        disabled={disabled}
        onClick={onClose}
      >
        Cancel
      </button>
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
        Create
      </button>
    </div>
  );
}
