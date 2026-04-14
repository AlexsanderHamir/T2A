import { Link } from "react-router-dom";
import type { Task } from "@/types";
import {
  EmptyState,
  EmptyStateSubtasksGlyph,
} from "@/shared/EmptyState";
import { priorityPillClass, statusPillClass } from "../../taskPillClasses";

export function SubtaskTree({
  nodes,
  nested = false,
  showNested = true,
}: {
  nodes: Task[];
  nested?: boolean;
  showNested?: boolean;
}) {
  if (!nodes.length) {
    if (nested) return null;
    return (
      <EmptyState
        id="task-subtasks-empty"
        density="compact"
        className="task-detail-section-empty"
        icon={<EmptyStateSubtasksGlyph />}
        title="No subtasks yet"
        description={
          <>
            Use <strong>Add subtask</strong> above to break work into smaller
            steps.
          </>
        }
      />
    );
  }
  return (
    <ul
      className={
        nested
          ? "task-subtasks-list task-subtasks-list--nested"
          : "task-subtasks-list"
      }
      aria-labelledby={nested ? undefined : "task-subtasks-heading"}
    >
      {nodes.map((c) => (
        <li key={c.id} className="task-subtasks-item">
          <div className="task-subtasks-item-row">
            <Link className="task-subtasks-link" to={`/tasks/${c.id}`}>
              {c.title}
            </Link>
            <div
              className="task-subtasks-item-meta"
              aria-label={`${c.title}: ${c.priority} priority, ${c.status}`}
            >
              <span className={priorityPillClass(c.priority)}>{c.priority}</span>
              <span className={statusPillClass(c.status)}>{c.status}</span>
            </div>
          </div>
          {showNested ? (
            <SubtaskTree nodes={c.children ?? []} nested showNested={showNested} />
          ) : null}
        </li>
      ))}
    </ul>
  );
}
