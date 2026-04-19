import { Link } from "react-router-dom";
import { FieldRequirementBadge } from "@/shared/FieldLabel";

type Props = {
  taskId: string;
  saving: boolean;
  onAddSubtask: () => void;
};

export function TaskDetailSubtasksHead({
  taskId,
  saving,
  onAddSubtask,
}: Props) {
  return (
    <div className="task-detail-subtasks-head">
      <div className="field-heading-with-req task-detail-subtasks-title-row">
        <h3
          className="task-detail-section-heading term-prompt"
          id="task-subtasks-heading"
        >
          <span>Subtasks</span>
        </h3>
        <FieldRequirementBadge requirement="optional" />
      </div>
      <div className="task-detail-subtasks-actions">
        <Link
          to={`/tasks/${encodeURIComponent(taskId)}/graph`}
          className="task-detail-open-graph-btn"
        >
          Open graph view
        </Link>
        <button
          type="button"
          className="task-detail-add-subtask-btn task-detail-add-subtask-btn--primary"
          onClick={onAddSubtask}
          disabled={saving}
        >
          Add subtask
        </button>
      </div>
    </div>
  );
}
