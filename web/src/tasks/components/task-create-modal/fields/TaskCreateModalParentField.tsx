import type { TaskWithDepth } from "../../../task-tree";
import { ParentTaskSelect } from "../../task-compose";

type Props = {
  parentOptionsLoading: boolean;
  parentId: string;
  parentOptions: TaskWithDepth[];
  onParentIdChange: (id: string) => void;
  disabled: boolean;
  hasParent: boolean;
};

export function TaskCreateModalParentField({
  parentOptionsLoading,
  parentId,
  parentOptions,
  onParentIdChange,
  disabled,
  hasParent,
}: Props) {
  return (
    <div className="task-create-parent-field grow">
      {parentOptionsLoading ? (
        <div className="task-create-parent-loading" aria-hidden="true">
          <span className="skeleton-block task-create-parent-loading-label" />
          <span className="skeleton-block task-create-parent-loading-input" />
        </div>
      ) : (
        <ParentTaskSelect
          id="task-new-parent"
          value={parentId}
          parentOptions={parentOptions}
          onChange={onParentIdChange}
          disabled={disabled}
        />
      )}
      <p className="task-create-parent-hint muted">
        {hasParent ? (
          <>
            Prompt, priority, and optional criteria — or inherit the parent&apos;s
            checklist.
          </>
        ) : (
          <>
            Empty = top-level task. Pick a parent to add a{" "}
            <strong>subtask</strong>.
          </>
        )}
      </p>
      {parentOptionsLoading ? (
        <p className="visually-hidden" role="status" aria-live="polite">
          Loading parent task options…
        </p>
      ) : null}
    </div>
  );
}
