import type { TaskGate } from "@/types";
import { taskGateStatusLabel } from "../../../task-display/taskGateDisplay";

type GateAction = "release" | "hold" | "clear_hold";

type Props = {
  gate: TaskGate | null | undefined;
  editable?: boolean;
  onAction?: (action: GateAction) => void;
  actionPending?: boolean;
  error?: string | null;
};

export function TaskGatePanel({
  gate,
  editable = false,
  onAction,
  actionPending = false,
  error = null,
}: Props) {
  if (!gate) {
    return (
      <section
        className="task-detail-section"
        id="task-detail-gate"
        aria-labelledby="task-detail-gate-title"
      >
        <h3
          id="task-detail-gate-title"
          className="task-detail-section-heading"
        >
          Release gate
        </h3>
        <p className="task-detail-empty-hint" data-testid="task-gate-empty">
          No gate on this task.
        </p>
      </section>
    );
  }

  const status = gate.status;
  const showHold =
    editable && status === "pending_release" && onAction && !gate.hold;
  const showClearHold =
    editable && status === "pending_release" && onAction && gate.hold;
  const showRelease =
    editable && status === "pending_release" && onAction;

  return (
    <section
      className="task-detail-section"
      id="task-detail-gate"
      aria-labelledby="task-detail-gate-title"
    >
      <h3
        id="task-detail-gate-title"
        className="task-detail-section-heading"
      >
        Release gate
      </h3>
      <div className="task-detail-meta" data-testid="task-gate-meta">
        <span className={`pd__chip pd__chip--gate pd__chip--${status}`}>
          {taskGateStatusLabel(status)}
        </span>
        {gate.hold ? <span className="pd__chip pd__chip--hold">On hold</span> : null}
      </div>
      {gate.pending_release_deadline ? (
        <p className="task-detail-gate-deadline">
          Pending release deadline: {gate.pending_release_deadline}
        </p>
      ) : null}
      {gate.criteria && gate.criteria.length > 0 ? (
        <ul className="task-gate-criteria" data-testid="task-gate-criteria">
          {gate.criteria.map((c) => (
            <li key={c.id} className={c.done ? "task-gate-criteria__done" : undefined}>
              {c.text}
            </li>
          ))}
        </ul>
      ) : null}
      {editable && onAction ? (
        <div className="task-gate-actions">
          {showHold ? (
            <button
              type="button"
              className="secondary"
              disabled={actionPending}
              onClick={() => onAction("hold")}
            >
              Hold release
            </button>
          ) : null}
          {showClearHold ? (
            <button
              type="button"
              className="secondary"
              disabled={actionPending}
              onClick={() => onAction("clear_hold")}
            >
              Clear hold
            </button>
          ) : null}
          {showRelease ? (
            <button
              type="button"
              className="primary"
              disabled={actionPending}
              onClick={() => onAction("release")}
            >
              Release gate
            </button>
          ) : null}
        </div>
      ) : null}
      {error ? (
        <p className="err" role="alert">
          {error}
        </p>
      ) : null}
    </section>
  );
}
