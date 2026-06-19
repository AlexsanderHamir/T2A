import type { Status, Task } from "@/types";
import { useAppTimezone, formatInAppTimezone } from "@/shared/time/appTimezone";

type Props = {
  task: Pick<Task, "status" | "pickup_not_before" | "criteria_satisfied_at">;
};

const TERMINAL_STATUSES: ReadonlySet<Status> = new Set(["done", "failed"]);

/**
 * Read-only pickup schedule line on the task detail toolbar. Mutations
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
    <div
      className="task-detail-schedule"
      data-testid="task-detail-schedule"
      data-state={hasSchedule ? "scheduled" : "unscheduled"}
    >
      {hasPhaseComplete ? (
        <span
          className="task-detail-schedule-badge task-detail-schedule-badge--phase"
          data-testid="task-detail-phase-complete"
        >
          <span aria-hidden="true" className="task-detail-schedule-badge-dot" />
          <span className="task-detail-schedule-badge-text">
            Phase complete
            <span className="task-detail-schedule-badge-sep" aria-hidden="true">
              ·
            </span>
            <time>{phaseFormatted}</time>
          </span>
        </span>
      ) : null}
      {hasSchedule ? (
        <span
          className="task-detail-schedule-badge"
          data-testid="task-detail-schedule-badge"
        >
          <span aria-hidden="true" className="task-detail-schedule-badge-dot" />
          <span className="task-detail-schedule-badge-text">
            Scheduled for
            <span className="task-detail-schedule-badge-sep" aria-hidden="true">
              ·
            </span>
            <time>{formatted}</time>
          </span>
        </span>
      ) : !hasPhaseComplete ? (
        <span className="task-detail-schedule-empty muted">
          No pickup scheduled.
        </span>
      ) : null}
    </div>
  );
}
