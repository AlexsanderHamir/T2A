import { Link } from "react-router-dom";

/** Matches `userAttention` return shape from `task-display/taskAttention.ts`. */
export type TaskDetailAttention = {
  show: boolean;
  headline: string;
  body: string;
};

type Props = {
  attention: TaskDetailAttention;
  saving: boolean;
  onEdit: () => void;
  onDelete: () => void;
  /** When set, shows "Run again" to requeue the task for the agent (PATCH status → ready). */
  onRequeue?: () => void;
  requeuePending?: boolean;
  /** Link to Settings → Cursor agent (model / CLI) after a failed run. */
  failedRunnerHint?: boolean;
};

export function TaskDetailAttentionBar({
  attention,
  saving,
  onEdit,
  onDelete,
  onRequeue,
  requeuePending,
  failedRunnerHint,
}: Props) {
  return (
    <>
      {attention.show ? (
        <div
          className="task-detail-attention"
          role="status"
          aria-live="polite"
        >
          <strong>{attention.headline}</strong>
          <p>{attention.body}</p>
        </div>
      ) : (
        <div className="task-detail-ok" role="status">
          <strong>No agent is waiting on you for this task right now.</strong>
          <p className="muted">
            Follow the timeline for updates. We highlight when an agent needs
            input or approval.
          </p>
        </div>
      )}

      <div className="task-detail-actions">
        {onRequeue ? (
          <button
            type="button"
            className="task-detail-btn-requeue"
            onClick={onRequeue}
            disabled={saving || requeuePending}
          >
            {requeuePending ? "Queueing…" : "Run again"}
          </button>
        ) : null}
        <button
          type="button"
          className="task-detail-btn-edit"
          onClick={onEdit}
          disabled={saving}
        >
          Edit task
        </button>
        <button
          type="button"
          className="task-detail-btn-delete"
          onClick={onDelete}
          disabled={saving}
        >
          Delete
        </button>
      </div>

      {failedRunnerHint ? (
        <p className="muted task-detail-runner-hint">
          <strong className="task-detail-runner-hint-label">Global:</strong>{" "}
          <Link to="/settings#cursor-agent">Cursor agent settings</Link> — CLI
          path and workspace default model.{" "}
          <strong className="task-detail-runner-hint-label">This task:</strong>{" "}
          use <strong>Edit task</strong> → Agent → Model to override the model
          for this task only. Headless runs use <code>--force</code> so tools
          auto-approve.
        </p>
      ) : null}
    </>
  );
}
