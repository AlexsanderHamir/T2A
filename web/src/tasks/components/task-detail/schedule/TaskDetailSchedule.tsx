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
        <div
          className="task-detail-schedule-row task-detail-schedule-row--phase"
          data-testid="task-detail-phase-complete"
          aria-label={`Phase completed, ${phaseFormatted}`}
        >
          <span className="task-detail-schedule-row-icon" aria-hidden="true">
            <PhaseCompleteGlyph />
          </span>
          <div className="task-detail-schedule-row-body">
            <span className="task-detail-schedule-row-label">Completed</span>
            <span className="task-detail-schedule-row-sep" aria-hidden="true">
              ·
            </span>
            <time dateTime={phaseCompleteAt}>{phaseFormatted}</time>
          </div>
        </div>
      ) : null}
      {hasSchedule ? (
        <div
          className="task-detail-schedule-row task-detail-schedule-row--scheduled"
          data-testid="task-detail-schedule-badge"
          aria-label={`Scheduled for pickup, ${formatted}`}
        >
          <span className="task-detail-schedule-row-icon" aria-hidden="true">
            <ScheduleGlyph />
          </span>
          <div className="task-detail-schedule-row-body">
            <span className="task-detail-schedule-row-label">Scheduled</span>
            <span className="task-detail-schedule-row-sep" aria-hidden="true">
              ·
            </span>
            <time dateTime={task.pickup_not_before}>{formatted}</time>
          </div>
        </div>
      ) : !hasPhaseComplete ? (
        <span className="task-detail-schedule-empty muted">
          No pickup scheduled.
        </span>
      ) : null}
    </div>
  );
}

function PhaseCompleteGlyph() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <circle cx="8" cy="8" r="6.25" />
      <path d="M5.25 8.25 7 10l3.75-4" />
    </svg>
  );
}

function ScheduleGlyph() {
  return (
    <svg
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.4"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <rect x="2.25" y="3.5" width="11.5" height="10.25" rx="2" />
      <path d="M2.25 6.5h11.5" />
      <path d="M5.5 2v3" />
      <path d="M10.5 2v3" />
    </svg>
  );
}
