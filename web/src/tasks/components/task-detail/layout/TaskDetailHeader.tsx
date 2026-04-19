import { Link } from "react-router-dom";
import type { Task } from "@/types";
import {
  priorityPillClass,
  statusNeedsUserInput,
  statusPillClass,
} from "../../../task-display";

const RUNNER_LABELS: Record<string, string> = {
  cursor: "Cursor CLI",
};

type TaskDetailHeaderTask = Pick<
  Task,
  "title" | "status" | "priority" | "runner" | "cursor_model"
>;

type Props = {
  task: TaskDetailHeaderTask;
};

export function TaskDetailHeader({ task }: Props) {
  const needsUser = statusNeedsUserInput(task.status);
  return (
    <>
      <nav className="task-detail-nav" aria-label="Task navigation">
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
      </nav>

      <header className="task-detail-header">
        <h2 className="task-detail-title term-arrow">
          <span>{task.title}</span>
        </h2>
        <p
          className="task-event-detail-stance"
          role="status"
          data-stance={needsUser ? "needs-user" : "informational"}
        >
          {needsUser ? "Agent needs input" : "Informational"}
        </p>
        <div className="task-detail-meta">
          <span
            className={statusPillClass(task.status)}
            data-needs-user={needsUser ? "true" : undefined}
          >
            {task.status}
          </span>
          <span className={priorityPillClass(task.priority)}>
            {task.priority}
          </span>
        </div>
        <p className="muted task-detail-agent-meta" aria-label="Agent for this task">
          {RUNNER_LABELS[task.runner] ?? task.runner}
          {(task.cursor_model ?? "").trim()
            ? ` · model ${task.cursor_model}`
            : " · default model"}
        </p>
      </header>
    </>
  );
}
