type Props = {
  saving: boolean;
  onEdit: () => void;
  onDelete: () => void;
  /** When set, shows retry actions for a failed task (POST /retry). */
  onRetryFresh?: () => void;
  onRetryResume?: () => void;
  retryPending?: boolean;
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

export function TaskDetailToolbarActions({
  saving,
  onEdit,
  onDelete,
  onRetryFresh,
  onRetryResume,
  retryPending,
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
    <div className="task-detail-actions">
      {onRetryFresh ? (
        <button
          type="button"
          className="task-detail-btn-retry-fresh"
          onClick={onRetryFresh}
          disabled={saving || retryPending}
        >
          {retryPending ? "Queueing…" : "Start over"}
        </button>
      ) : null}
      {onRetryResume ? (
        <button
          type="button"
          className="task-detail-btn-retry-resume"
          onClick={onRetryResume}
          disabled={saving || retryPending}
        >
          Resume from failure
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
  );
}

/** @deprecated Use `TaskDetailToolbarActions` — toolbar no longer renders status copy. */
export const TaskDetailAttentionBar = TaskDetailToolbarActions;

export type TaskDetailAttentionBarProps = Props;
