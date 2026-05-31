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
  /**
   * When set, shows the "Model configuration" action which opens the
   * model-configuration modal (consolidates the failure-recovery hint
   * that used to live inline below the action row).
   */
  onConfigureModel?: () => void;
  /**
   * Gates whether the "Model configuration" action is offered at all.
   * Today it is offered after a failed run; older copy referred to this
   * as `failedRunnerHint`.
   */
  showModelConfig?: boolean;
  /**
   * Autonomous-execution mode for this task. `"hidden"` suppresses the
   * toggle entirely (e.g. running, done, failed — the autonomy concept
   * does not apply). `"ready"` shows a "Put on hold" action; `"on_hold"`
   * shows a "Resume" action. Both actions go through a confirm dialog
   * upstream of `onToggleAutonomy`.
   */
  autonomyMode?: "hidden" | "ready" | "on_hold";
  onToggleAutonomy?: () => void;
  autonomyPending?: boolean;
};

export function TaskDetailAttentionBar({
  attention,
  saving,
  onEdit,
  onDelete,
  onRequeue,
  requeuePending,
  onConfigureModel,
  showModelConfig,
  autonomyMode = "hidden",
  onToggleAutonomy,
  autonomyPending = false,
}: Props) {
  const showAutonomy =
    autonomyMode !== "hidden" && typeof onToggleAutonomy === "function";
  const autonomyLabel =
    autonomyMode === "on_hold" ? "Resume" : "Put on hold";
  const autonomyPendingLabel =
    autonomyMode === "on_hold" ? "Resuming…" : "Holding…";
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
        {showAutonomy ? (
          <button
            type="button"
            className="task-detail-btn-autonomy"
            onClick={onToggleAutonomy}
            disabled={saving || autonomyPending}
            data-autonomy-mode={autonomyMode}
          >
            {autonomyPending ? autonomyPendingLabel : autonomyLabel}
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
        {showModelConfig && onConfigureModel ? (
          <button
            type="button"
            className="task-detail-btn-model-config"
            onClick={onConfigureModel}
            disabled={saving}
          >
            Model configuration
          </button>
        ) : null}
        <button
          type="button"
          className="task-detail-btn-delete"
          onClick={onDelete}
          disabled={saving}
        >
          Delete
        </button>
      </div>
    </>
  );
}
