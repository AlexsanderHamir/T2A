import { Link } from "react-router-dom";
import type { Task } from "@/types";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";

export function SubtaskTree({
  nodes,
  nested = false,
}: {
  nodes: Task[];
  nested?: boolean;
}) {
  if (!nodes.length) {
    if (nested) return null;
    return (
      <p className="muted task-subtasks-empty" id="task-subtasks-empty">
        No subtasks yet. Use{" "}
        <span className="task-subtasks-empty-accent">Add subtask</span> to break
        work into smaller steps.
      </p>
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
          <SubtaskTree nodes={c.children ?? []} nested />
        </li>
      ))}
    </ul>
  );
}
