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
  /** Opens the change-model-only modal (not full edit). Shown in model configuration row. */
  onChangeModel?: () => void;
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
  onChangeModel,
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
        <section
          className="task-detail-model-config"
          aria-labelledby="task-detail-model-config-title"
        >
          <div className="task-detail-model-config-inner">
            <h3
              className="task-detail-model-config-title"
              id="task-detail-model-config-title"
            >
              Model configuration
            </h3>
            <div className="task-detail-model-config-body">
              <div className="task-detail-model-config-row">
                <div className="task-detail-model-config-copy">
                  <span className="task-detail-model-config-row-title">
                    Global model
                  </span>
                  <span className="task-detail-model-config-row-hint">
                    All tasks in this workspace
                  </span>
                </div>
                <div className="task-detail-model-config-actions">
                  <Link
                    to="/settings#cursor-agent"
                    className="task-detail-agent-model-cta"
                  >
                    Global agent settings
                  </Link>
                </div>
              </div>
              <div
                className="task-detail-model-config-divider"
                role="presentation"
              />
              <div className="task-detail-model-config-row">
                <div className="task-detail-model-config-copy">
                  <span className="task-detail-model-config-row-title">
                    Per-task model
                  </span>
                  <span className="task-detail-model-config-row-hint">
                    This task only
                  </span>
                </div>
                <div className="task-detail-model-config-actions">
                  <button
                    type="button"
                    className="task-detail-agent-model-cta"
                    onClick={onChangeModel ?? onEdit}
                    disabled={saving}
                    aria-label="Change per-task model"
                  >
                    Change model
                  </button>
                </div>
              </div>
            </div>
          </div>
        </section>
      ) : null}
    </>
  );
}
