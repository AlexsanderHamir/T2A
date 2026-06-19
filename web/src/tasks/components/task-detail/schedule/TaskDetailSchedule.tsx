import type { Status, Task } from "@/types";
import { useAppTimezone, formatInAppTimezone } from "@/shared/time/appTimezone";

type Props = {
  task: Pick<Task, "status" | "pickup_not_before" | "criteria_satisfied_at">;
};

const TERMINAL_STATUSES: ReadonlySet<Status> = new Set(["done", "failed"]);

/**
 * Read-only pickup schedule rows on the task detail toolbar card. Mutations
 * live in the edit-task modal (`TaskCreateModal` edit mode + `SchedulePicker`).
 */
export function TaskDetailSchedule({ task }: Props) {
  const tz = useAppTimezone();
  const isTerminal = TERMINAL_STATUSES.has(task.status);
  const hasSchedule = Boolean(task.pickup_not_before);
  const phaseCompleteAt = (task.criteria_satisfied_at ?? "").trim();
  const hasPhaseComplete = phaseCompleteAt !== "";

  if (!hasSchedule && isTerminal && !hasPhaseComplete) {
    return null;
  }

  const formatted = task.pickup_not_before
    ? formatInAppTimezone(task.pickup_not_before, tz)
    : null;
  const phaseFormatted = hasPhaseComplete
    ? formatInAppTimezone(phaseCompleteAt, tz)
    : null;

  return (
    <ul
      className="task-detail-schedule"
      data-testid="task-detail-schedule"
      data-state={hasSchedule ? "scheduled" : "unscheduled"}
    >
      {hasPhaseComplete ? (
        <li
          className="task-detail-fact-row task-detail-schedule-row"
          data-testid="task-detail-phase-complete"
        >
          <span className="cell-pill cell-pill--commit-eligible">
            Phase complete
          </span>
          <time className="task-detail-fact-when muted">{phaseFormatted}</time>
        </li>
      ) : null}
      {hasSchedule ? (
        <li
          className="task-detail-fact-row task-detail-schedule-row"
          data-testid="task-detail-schedule-badge"
        >
          <span className="cell-pill task-detail-schedule-pill">
            Scheduled for
          </span>
          <time className="task-detail-fact-when muted">{formatted}</time>
        </li>
      ) : !hasPhaseComplete ? (
        <li className="task-detail-fact-row task-detail-schedule-row">
          <span className="task-detail-schedule-empty muted">
            No pickup scheduled.
          </span>
        </li>
      ) : null}
    </ul>
  );
}
