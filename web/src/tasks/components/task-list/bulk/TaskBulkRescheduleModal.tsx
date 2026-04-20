import { useCallback, useEffect, useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { SchedulePicker } from "@/shared/time/SchedulePicker";

type Props = {
  /**
   * Number of tasks the operator selected when they opened the
   * modal. Surfaced in the title and the submit button so the
   * operator never loses sight of the blast radius. We pass an
   * explicit count rather than the full id list because the
   * modal itself is purely presentational; the parent owns the
   * selection state and the PATCH dispatch.
   */
  selectedCount: number;
  appTimezone: string;
  /**
   * Whether the underlying bulk PATCH is in flight. The picker
   * disables itself and the buttons render their busy copy while
   * true.
   */
  busy: boolean;
  /** Surfaces the most recent bulk-mutation error, if any. */
  error?: Error | string | null;
  onClose: () => void;
  /**
   * Called with the picker's emitted value (RFC3339 UTC ISO or
   * `null` when the operator cleared the input). The parent is
   * responsible for actually firing the bulk PATCH and closing
   * the modal on success.
   */
  onSubmit: (next: string | null) => void;
};

/**
 * TaskBulkRescheduleModal — modal wrapper around the shared
 * `SchedulePicker` for the list-page bulk action. Pure presentation:
 * owns the draft schedule state, surfaces the operator-chosen
 * timezone, and forwards the picker's value to the parent on
 * Save. The parent (`TaskListSection`) owns the selected ids,
 * the PATCH mutation, and decides when to close the modal
 * (typically on `BulkScheduleResult.failed.length === 0`, but the
 * parent may choose to leave it open after a partial failure so
 * the operator can retry).
 */
export function TaskBulkRescheduleModal({
  selectedCount,
  appTimezone,
  busy,
  error,
  onClose,
  onSubmit,
}: Props) {
  const [draft, setDraft] = useState<string | null>(null);

  // Reset the draft any time the modal "opens fresh" — i.e. the
  // selectedCount changes or the appTimezone flips. Operators
  // expect a clean slate every time they click Reschedule; carrying
  // a stale draft over from a previous selection would be a UX
  // trap.
  useEffect(() => {
    setDraft(null);
  }, [selectedCount, appTimezone]);

  const handleSubmit = useCallback(() => {
    onSubmit(draft);
  }, [draft, onSubmit]);

  return (
    <Modal
      onClose={onClose}
      labelledBy="task-bulk-reschedule-title"
      busy={busy}
      busyLabel="Rescheduling tasks…"
      dismissibleWhileBusy
    >
      <section className="panel modal-sheet task-bulk-reschedule-modal-sheet">
        <h2 id="task-bulk-reschedule-title">
          Reschedule {selectedCount}{" "}
          {selectedCount === 1 ? "task" : "tasks"}
        </h2>
        <p className="muted">
          Pick a future pickup time for the selected{" "}
          {selectedCount === 1 ? "task" : "tasks"}. Times are shown in{" "}
          <strong>{appTimezone}</strong>. Clearing the picker before saving
          unschedules all selected tasks (same effect as the bar's
          "Clear schedule" action).
        </p>
        <SchedulePicker
          value={draft}
          onChange={setDraft}
          appTimezone={appTimezone}
          disabled={busy}
          idPrefix="task-bulk-reschedule"
        />
        <MutationErrorBanner
          error={error}
          fallback="Could not reschedule the selected tasks."
          className="task-bulk-reschedule-modal-err"
        />
        <div className="row stack-row-actions">
          <button
            type="button"
            className="secondary"
            onClick={onClose}
            disabled={busy}
          >
            Cancel
          </button>
          <button
            type="button"
            className="task-create-submit"
            onClick={handleSubmit}
            disabled={busy}
            data-testid="task-bulk-reschedule-submit"
          >
            {busy
              ? "Rescheduling…"
              : `Reschedule ${selectedCount}`}
          </button>
        </div>
      </section>
    </Modal>
  );
}
