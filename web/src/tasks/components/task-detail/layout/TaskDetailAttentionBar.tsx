/** Matches `userAttention` return shape from `task-display/taskAttention.ts`. */
export type TaskDetailAttention = {
  show: boolean;
  headline: string;
  body: string;
};

/**
 * Semantic tone for the "all clear" dot. The OK line carries a single
 * pre-defined copy line ("No agent is waiting on you…"), but the
 * *reason* there's nothing for the operator to do varies by task
 * status — done, actively running, queued, or operator-paused. The
 * dot colour encodes that reason at a glance so two tasks both
 * showing the OK line are distinguishable without reading status
 * pills:
 *
 * - `success` → done (task finished cleanly)
 * - `active`  → running (the agent is doing the work right now)
 * - `info`    → ready (queued for the agent to pick up)
 * - `caution` → on_hold (operator paused; nothing will happen)
 * - `neutral` → fallback for any other quiet state
 *
 * Tones map 1:1 to `data-tone` selectors in
 * `app-task-detail-model-config.css`.
 */
export type TaskDetailOkTone =
  | "success"
  | "active"
  | "info"
  | "caution"
  | "neutral";

type Props = {
  attention: TaskDetailAttention;
  saving: boolean;
  onEdit: () => void;
  onDelete: () => void;
  /**
   * Semantic colour for the OK-line dot. Only consulted when
   * `attention.show` is false (i.e. the OK line actually renders).
   * Defaults to `"neutral"` so callers that don't yet plumb a tone
   * still get a sensible grey.
   */
  okTone?: TaskDetailOkTone;
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
        // Happy path: the operator owns nothing here. A bordered card with
        // an icon ring overstated a non-event — the default state should
        // recede, not announce itself. One muted line keeps the signal
        // present without competing with the title or actions for attention.
        // `data-tone` colours the leading dot per task status (see
        // `TaskDetailOkTone`) so the same copy still distinguishes
        // "done" from "running" from "on_hold" at a glance.
        <p className="task-detail-ok" role="status" data-tone={okTone}>
          <span className="task-detail-ok-inner">
            <span className="task-detail-ok-icon" aria-hidden="true">
              <svg
                width={16}
                height={16}
                viewBox="0 0 24 24"
                fill="none"
                xmlns="http://www.w3.org/2000/svg"
              >
                <circle
                  cx={12}
                  cy={12}
                  r={9}
                  stroke="currentColor"
                  strokeWidth={1.75}
                />
                <path
                  d="M8 12.5 10.8 15.2 16 9.8"
                  stroke="currentColor"
                  strokeWidth={1.75}
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </span>
            <span className="task-detail-ok-copy">
              No agent is waiting on you for this task right now.
            </span>
          </span>
        </p>
      )}

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
    </>
  );
}
