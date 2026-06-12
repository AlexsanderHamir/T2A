import type { Status, Task } from "@/types";
import { useAppTimezone, formatInAppTimezone } from "@/shared/time/appTimezone";

type Props = {
  task: Pick<Task, "status" | "pickup_not_before">;
};

const TERMINAL_STATUSES: ReadonlySet<Status> = new Set(["done", "failed"]);

/**
 * Read-only pickup schedule line on the task detail toolbar. Mutations
 * live in the edit-task form (`TaskEditForm` + `SchedulePicker`).
 */
export function TaskDetailSchedule({ task }: Props) {
  const tz = useAppTimezone();
  const isTerminal = TERMINAL_STATUSES.has(task.status);
  const hasSchedule = Boolean(task.pickup_not_before);

  if (!hasSchedule && isTerminal) {
    return null;
  }

  const formatted = task.pickup_not_before
    ? formatInAppTimezone(task.pickup_not_before, tz)
    : null;

  return (
    <div
      className="task-detail-schedule"
      data-testid="task-detail-schedule"
      data-state={hasSchedule ? "scheduled" : "unscheduled"}
    >
      {hasSchedule ? (
        <span
          className="task-detail-schedule-badge"
          data-testid="task-detail-schedule-badge"
        >
          <span aria-hidden="true" className="task-detail-schedule-badge-dot" />
          <span className="task-detail-schedule-badge-text">
            Scheduled for {formatted}
          </span>
        </span>
      ) : (
        <span className="task-detail-schedule-empty muted">
          No pickup scheduled.
        </span>
      )}
    </div>
  );
}
