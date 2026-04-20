import { Link } from "react-router-dom";
import { cycleRunnerChipClass, runnerLabel } from "@/observability";
import type { Task } from "@/types";
import {
  priorityPillClass,
  statusNeedsUserInput,
  statusPillClass,
} from "../../../task-display";

type TaskDetailHeaderTask = Pick<
  Task,
  "title" | "status" | "priority" | "runner" | "cursor_model"
>;

type Props = {
  task: TaskDetailHeaderTask;
};

// formatTaskRuntime renders the header chip copy. Unlike
// `formatRunnerModel` (which reads `cursor_model_effective` off a
// cycle's meta — the truth the runner resolved), the header is about
// the task's INTENT: `task.cursor_model` is what the operator picked
// for the NEXT run, which may differ from any historical cycle's
// effective model. "default model" is the copy for the empty-intent
// case (the runner will fill in its adapter default at start).
function formatTaskRuntime(task: TaskDetailHeaderTask): string {
  const runner = runnerLabel(task.runner);
  if (runner === "unknown runner") {
    return runner;
  }
  const model = (task.cursor_model ?? "").trim();
  if (!model) {
    return `${runner} · default model`;
  }
  return `${runner} · ${model}`;
}

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
          <span
            className={`cell-pill ${cycleRunnerChipClass()}`}
            data-testid="task-detail-runtime"
            aria-label="Agent for this task"
          >
            {formatTaskRuntime(task)}
          </span>
        </div>
      </header>
    </>
  );
}
