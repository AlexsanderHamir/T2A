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
      <div className="task-detail-schedule-strip">
        {hasPhaseComplete ? (
          <span
            className="task-detail-schedule-badge task-detail-schedule-badge--phase"
            data-testid="task-detail-phase-complete"
          >
            <span aria-hidden="true" className="task-detail-schedule-badge-icon">
              <ScheduleStatusIcon variant="phase" />
            </span>
            <span className="task-detail-schedule-badge-body">
              <span className="task-detail-schedule-badge-label">
                Phase complete
              </span>
              <time className="task-detail-schedule-badge-value">
                {phaseFormatted}
              </time>
            </span>
          </span>
        ) : null}
        {hasSchedule ? (
          <span
            className="task-detail-schedule-badge"
            data-testid="task-detail-schedule-badge"
          >
            <span aria-hidden="true" className="task-detail-schedule-badge-icon">
              <ScheduleStatusIcon variant="scheduled" />
            </span>
            <span className="task-detail-schedule-badge-body">
              <span className="task-detail-schedule-badge-label">
                Scheduled for
              </span>
              <time className="task-detail-schedule-badge-value">
                {formatted}
              </time>
            </span>
          </span>
        ) : !hasPhaseComplete ? (
          <span className="task-detail-schedule-empty muted">
            No pickup scheduled.
          </span>
        ) : null}
      </div>
    </div>
  );
}

function ScheduleStatusIcon({
  variant,
}: {
  variant: "phase" | "scheduled";
}) {
  if (variant === "phase") {
    return (
      <svg
        width={14}
        height={14}
        viewBox="0 0 24 24"
        fill="none"
        xmlns="http://www.w3.org/2000/svg"
      >
        <path
          d="M20 6 9 17l-5-5"
          stroke="currentColor"
          strokeWidth={2}
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    );
  }

  return (
    <svg
      width={14}
      height={14}
      viewBox="0 0 24 24"
      fill="none"
      xmlns="http://www.w3.org/2000/svg"
    >
      <rect
        x={3}
        y={4}
        width={18}
        height={18}
        rx={2}
        stroke="currentColor"
        strokeWidth={1.75}
      />
      <path
        d="M16 2v4M8 2v4M3 10h18"
        stroke="currentColor"
        strokeWidth={1.75}
        strokeLinecap="round"
      />
    </svg>
  );
}
