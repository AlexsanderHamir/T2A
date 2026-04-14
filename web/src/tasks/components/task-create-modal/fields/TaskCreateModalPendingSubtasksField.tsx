import { FieldRequirementBadge } from "@/shared/FieldLabel";
import type { PendingSubtaskDraft } from "../../../pendingSubtaskDraft";

type Props = {
  pendingSubtasks: PendingSubtaskDraft[];
  disabled: boolean;
  onOpenNestedNew: () => void;
  onOpenNestedEdit: (index: number) => void;
  onRemovePendingSubtask: (index: number) => void;
};

const SUBTASKS_HEADING_ID = "task-new-subtasks-heading";

export function TaskCreateModalPendingSubtasksField({
  pendingSubtasks,
  disabled,
  onOpenNestedNew,
  onOpenNestedEdit,
  onRemovePendingSubtask,
}: Props) {
  return (
    <div className="task-create-subtasks">
      <div className="task-create-subtasks-head">
        <div className="field-heading-with-req task-create-subtasks-heading-row">
          <h3 className="task-create-subtasks-heading" id={SUBTASKS_HEADING_ID}>
            Subtasks
          </h3>
          <FieldRequirementBadge requirement="optional" />
        </div>
        <button
          type="button"
          className="task-detail-add-subtask-btn"
          disabled={disabled}
          aria-label="Open form to add a subtask"
          onClick={onOpenNestedNew}
        >
          New subtask
        </button>
      </div>
      <p className="task-create-subtasks-hint muted">
        <strong>New subtask</strong> opens another form. Subtasks are created
        when you click <strong>Create</strong>.
      </p>
      {pendingSubtasks.length > 0 ? (
        <ul className="task-checklist-list" aria-labelledby={SUBTASKS_HEADING_ID}>
          {pendingSubtasks.map((d, index) => (
            <li
              key={`${index}-${d.title}`}
              className="task-checklist-row task-create-pending-subtask-row"
            >
              <span className="task-checklist-label">{d.title}</span>
              <div className="task-create-pending-subtask-actions">
                <button
                  type="button"
                  className="task-detail-checklist-add-btn"
                  disabled={disabled}
                  onClick={() => onOpenNestedEdit(index)}
                >
                  Edit
                </button>
                <button
                  type="button"
                  className="task-create-checklist-remove"
                  disabled={disabled}
                  onClick={() => onRemovePendingSubtask(index)}
                >
                  Remove
                </button>
              </div>
            </li>
          ))}
        </ul>
      ) : null}
    </div>
  );
}
