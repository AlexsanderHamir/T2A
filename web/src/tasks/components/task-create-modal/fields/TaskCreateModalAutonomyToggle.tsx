import type { ChangeEvent } from "react";

type Props = {
  enabled: boolean;
  disabled: boolean;
  onChange: (enabled: boolean) => void;
};

/**
 * Autonomy gate for the create-task modal.
 *
 * When `enabled` is false the parent flow submits the task with
 * `status: "on_hold"`. The agent worker picks up only `status: "ready"`
 * tasks (ReadyForAgentPickup, pkgs/tasks/store/internal/tasks/readiness.go),
 * so on-hold tasks sit untouched until the operator resumes them from
 * the task detail page.
 *
 * Sits between the primary fields and the "More options" details block
 * because it changes the most fundamental thing about the new task —
 * whether the agent is allowed to start working on it. Hiding this
 * behind "More options" buries a primary intent.
 */
export function TaskCreateModalAutonomyToggle({
  enabled,
  disabled,
  onChange,
}: Props) {
  function handle(e: ChangeEvent<HTMLInputElement>) {
    onChange(e.target.checked);
  }
  return (
    <section className="task-create-autonomy" aria-label="Autonomous execution">
      <label
        className="task-create-autonomy__row"
        htmlFor="task-create-autonomy-toggle"
      >
        <span className="task-create-autonomy__text">
          <span className="task-create-autonomy__label">
            Autonomous execution
          </span>
          <span className="task-create-autonomy__hint">
            {enabled
              ? "The agent will pick this task up when its scheduling and dependencies allow."
              : "The task will be created on hold. The agent will not pick it up until you resume it from the task detail page."}
          </span>
        </span>
        <input
          id="task-create-autonomy-toggle"
          type="checkbox"
          className="task-create-autonomy__input"
          checked={enabled}
          disabled={disabled}
          onChange={handle}
        />
      </label>
    </section>
  );
}
