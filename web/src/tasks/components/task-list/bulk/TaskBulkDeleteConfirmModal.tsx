import { useEffect, useRef } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";

export type TaskBulkDeleteRow = {
  id: string;
  title: string;
  descendantCount?: number;
};

type Props = {
  tasks: readonly TaskBulkDeleteRow[];
  busy: boolean;
  error?: string | null;
  onCancel: () => void;
  onConfirm: () => void;
};

/**
 * Confirmation for deleting N list rows. Each DELETE cascades on the
 * server; we sum `descendantCount` across selected roots to surface the
 * same subtree warning as the single-row dialog.
 */
export function TaskBulkDeleteConfirmModal({
  tasks,
  busy,
  error = null,
  onCancel,
  onConfirm,
}: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null);
  const count = tasks.length;
  const noun = count === 1 ? "task" : "tasks";
  const totalSubtasks = tasks.reduce(
    (s, t) => s + (t.descendantCount ?? 0),
    0,
  );

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  return (
    <Modal
      onClose={onCancel}
      labelledBy="task-bulk-delete-title"
      describedBy="task-bulk-delete-description"
      busy={busy}
      busyLabel="Deleting tasks…"
      dismissibleWhileBusy
    >
      <section className="panel confirm-dialog modal-sheet">
        <h2 id="task-bulk-delete-title">Delete {count} {noun}?</h2>
        <p
          className="confirm-dialog__statement"
          id="task-bulk-delete-description"
        >
          {count === 1 ? (
            <>
              <strong>{tasks[0]?.title ?? "This task"}</strong> will be
              permanently deleted.
            </>
          ) : (
            <>
              The {count} selected tasks will be permanently deleted.
            </>
          )}
        </p>
        {totalSubtasks > 0 ? (
          <div className="confirm-dialog__cascade" role="note">
            <svg
              className="confirm-dialog__cascade-icon"
              viewBox="0 0 16 16"
              fill="currentColor"
              aria-hidden="true"
              focusable="false"
            >
              <path d="M7.13 2.34a1 1 0 0 1 1.74 0l5.7 9.86A1 1 0 0 1 13.7 13.7H2.3a1 1 0 0 1-.87-1.5l5.7-9.86Zm.87 3.41a.75.75 0 0 0-.75.75v3a.75.75 0 0 0 1.5 0v-3a.75.75 0 0 0-.75-.75Zm0 6.25a.9.9 0 1 0 0-1.8.9.9 0 0 0 0 1.8Z" />
            </svg>
            <span>
              {totalSubtasks === 1 ? (
                <>
                  <strong>1 subtask</strong> under the selected{" "}
                  {noun} will also be removed (server cascade).
                </>
              ) : (
                <>
                  <strong>{totalSubtasks} subtasks</strong> under the
                  selected {noun} will also be removed (server cascade).
                </>
              )}
            </span>
          </div>
        ) : null}
        <p className="confirm-dialog__footnote">This action cannot be undone.</p>
        <MutationErrorBanner error={error} className="confirm-dialog__err" />
        <div className="row stack-row-actions">
          <button
            ref={cancelRef}
            type="button"
            className="secondary"
            disabled={busy}
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type="button"
            className="danger"
            disabled={busy}
            onClick={() => void onConfirm()}
            data-testid="task-bulk-delete-confirm"
          >
            {busy ? "Deleting…" : `Delete ${count}`}
          </button>
        </div>
      </section>
    </Modal>
  );
}
