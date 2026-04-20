type Props = {
  selectedCount: number;
  /**
   * How many of the currently selected rows already carry a
   * schedule. Drives the "Clear schedule" affordance: if zero
   * selected rows have a schedule, clearing is a no-op so we
   * disable the button and label it accordingly.
   */
  scheduledCount: number;
  busy: boolean;
  onReschedule: () => void;
  onClearSchedule: () => void;
  onCancel: () => void;
};

/**
 * TaskListBulkActionBar — sticky bottom action bar that appears
 * whenever the operator has at least one selected row. Lives at
 * the bottom of the table so it never covers the rows being
 * acted upon.
 *
 * Three actions:
 *  - **Reschedule** (primary): opens TaskBulkRescheduleModal with
 *    the shared SchedulePicker. Always enabled when the bar is
 *    visible.
 *  - **Clear schedule** (secondary): immediate PATCH N times with
 *    `pickup_not_before: null`. Disabled when none of the
 *    selected rows have a schedule (the operator's mental model
 *    is "remove the deferred-pickup time"; if nothing has one,
 *    there's nothing to clear). For N > 5 the parent renders a
 *    `confirm()` step before firing.
 *  - **Cancel** (tertiary): clears the running selection without
 *    firing any PATCHes.
 *
 * Visibility (`selectedCount > 0`) and selection lifecycle
 * (clearing on filter/sort change or successful bulk action) are
 * the parent's concern; the bar is purely presentational.
 */
export function TaskListBulkActionBar({
  selectedCount,
  scheduledCount,
  busy,
  onReschedule,
  onClearSchedule,
  onCancel,
}: Props) {
  if (selectedCount === 0) return null;
  const noun = selectedCount === 1 ? "task" : "tasks";
  return (
    <div
      className="task-list-bulk-bar"
      role="toolbar"
      aria-label="Bulk actions for selected tasks"
      data-testid="task-list-bulk-bar"
    >
      <span
        className="task-list-bulk-bar-summary"
        aria-live="polite"
        data-testid="task-list-bulk-bar-summary"
      >
        {selectedCount} {noun} selected
      </span>
      <div className="task-list-bulk-bar-actions">
        <button
          type="button"
          className="task-create-submit"
          onClick={onReschedule}
          disabled={busy}
          data-testid="task-list-bulk-bar-reschedule"
        >
          Reschedule
        </button>
        <button
          type="button"
          className="secondary"
          onClick={onClearSchedule}
          disabled={busy || scheduledCount === 0}
          title={
            scheduledCount === 0
              ? "None of the selected tasks have a schedule to clear."
              : `Clear scheduled pickup on ${scheduledCount} selected ${
                  scheduledCount === 1 ? "task" : "tasks"
                }.`
          }
          data-testid="task-list-bulk-bar-clear"
        >
          Clear schedule
        </button>
        <button
          type="button"
          className="secondary"
          onClick={onCancel}
          disabled={busy}
          data-testid="task-list-bulk-bar-cancel"
        >
          Cancel
        </button>
      </div>
    </div>
  );
}
