import { Link } from "react-router-dom";
import type { Task } from "@/types";
import { priorityPillClass, statusPillClass } from "../../../task-display";

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
    // Quiet empty hint that mirrors Dependencies / Release Gate.
    // The previous treatment was a full `<EmptyState>` card with an
    // icon glyph + h2 title + description body, which made the
    // empty subtasks slot the loudest section on the page — taller
    // and bolder than the populated Dependencies or Gate sections
    // above it, even though "no subtasks" is the least eventful
    // possible state. A single muted line matches the established
    // section-empty rhythm: one fact, no chrome.
    return (
      <p
        className="task-detail-empty-hint"
        id="task-subtasks-empty"
        data-testid="task-subtasks-empty"
      >
        No subtasks yet. Use Add subtask to break work into smaller steps.
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
          {showNested ? (
            <SubtaskTree nodes={c.children ?? []} nested showNested={showNested} />
          ) : null}
        </li>
      ))}
    </ul>
  );
}
