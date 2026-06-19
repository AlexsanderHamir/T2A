/** Matches `userAttention` return shape from `task-display/taskAttention.ts`. */
export type TaskDetailAttention = {
  show: boolean;
  headline: string;
  body: string;
};

/**
 * Semantic tone for the "all clear" pill. The OK line carries a single
 * pre-defined copy line ("No agent is waiting on you…"), but the
 * *reason* there's nothing for the operator to do varies by task
 * status — done, actively running, queued, or operator-paused.
 */
export type TaskDetailOkTone =
  | "success"
  | "active"
  | "info"
  | "caution"
  | "neutral";

type ActionProps = {
  saving: boolean;
  onEdit: () => void;
  onDelete: () => void;
  onRetryFresh?: () => void;
  onRetryResume?: () => void;
  retryPending?: boolean;
  onConfigureModel?: () => void;
  showModelConfig?: boolean;
  autonomyMode?: "hidden" | "ready" | "on_hold";
  onToggleAutonomy?: () => void;
  autonomyPending?: boolean;
};

type Props = ActionProps & {
  attention: TaskDetailAttention;
  okTone?: TaskDetailOkTone;
  /** When false, only the status/attention fact row renders (for toolbar card layout). */
  showActions?: boolean;
};

function okPillLabel(tone: TaskDetailOkTone): string {
  switch (tone) {
    case "success":
      return "All clear";
    case "active":
      return "Running";
    case "info":
      return "Ready";
    case "caution":
      return "On hold";
    default:
      return "All clear";
  }
}

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
}: ActionProps) {
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

export function TaskDetailAttentionBar({
  attention,
  saving,
  onEdit,
  onDelete,
  okTone = "neutral",
  onRetryFresh,
  onRetryResume,
  retryPending,
  onConfigureModel,
  showModelConfig,
  autonomyMode = "hidden",
  onToggleAutonomy,
  autonomyPending = false,
  showActions = true,
}: Props) {
  const actionProps: ActionProps = {
    saving,
    onEdit,
    onDelete,
    onRetryFresh,
    onRetryResume,
    retryPending,
    onConfigureModel,
    showModelConfig,
    autonomyMode,
    onToggleAutonomy,
    autonomyPending,
  };

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
        <div
          className="task-detail-fact-row task-detail-ok"
          role="status"
          data-tone={okTone}
        >
          <span
            className={`cell-pill task-detail-ok-pill task-detail-ok-pill--${okTone}`}
          >
            {okPillLabel(okTone)}
          </span>
          <span className="task-detail-fact-copy">
            No agent is waiting on you for this task right now.
          </span>
        </div>
      )}

      {showActions ? <TaskDetailToolbarActions {...actionProps} /> : null}
    </>
  );
}
