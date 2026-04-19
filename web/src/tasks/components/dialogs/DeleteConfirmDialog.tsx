import { useEffect, useRef } from "react";
import { Modal } from "../../../shared/Modal";

type Props = {
  taskTitle: string;
  saving: boolean;
  deletePending: boolean;
  /**
   * Total descendant count for the task being confirmed (children +
   * grandchildren …). When > 0 the dialog appends a cascade warning line so
   * the user knows the server-side `DELETE /tasks/{id}` will remove every
   * descendant in one transaction (docs/API-HTTP.md "DELETE /tasks/{id}").
   * Defaults to 0 when omitted so call-sites without a tree stay
   * source-compatible (no warning line rendered).
   */
  subtaskCount?: number;
  onCancel: () => void;
  onConfirm: () => void;
};

export function DeleteConfirmDialog({
  taskTitle,
  saving,
  deletePending,
  subtaskCount = 0,
  onCancel,
  onConfirm,
}: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  return (
    <Modal
      onClose={onCancel}
      labelledBy="delete-dialog-title"
      describedBy="delete-dialog-description"
      busy={deletePending}
      // The spinner still gives in-flight feedback, but the user can
      // step away (Escape / backdrop) without being trapped behind a
      // slow DELETE. Safe because `useTaskDeleteFlow.deleteTask.onSuccess`
      // is id-aware: it clears `deleteTarget` only when
      // `prev?.id === deletedId`, and the cross-cut `onDeleted(id)`
      // callback uses the same compare on `editing`. So a stale
      // resolution after the user dismissed (deleteTarget = null) or
      // queued a different deletion (deleteTarget = nextTask) cannot
      // clobber unrelated UI state. Server-truth invalidations
      // (`tasks/list`, `task-stats`) still fire so the deleted row
      // disappears from the list even when the dialog is already gone.
      dismissibleWhileBusy
    >
      <section className="panel confirm-dialog modal-sheet">
        <h2 id="delete-dialog-title">Delete this task?</h2>
        <p
          className="confirm-dialog__statement"
          id="delete-dialog-description"
        >
          <strong>{taskTitle}</strong> will be permanently deleted.
        </p>
        {subtaskCount > 0 ? (
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
              {subtaskCount === 1 ? (
                <>
                  <strong>1 subtask</strong> will also be deleted.
                </>
              ) : (
                <>
                  <strong>{subtaskCount} subtasks</strong> will also be deleted.
                </>
              )}
            </span>
          </div>
        ) : null}
        <p className="confirm-dialog__footnote">
          This action cannot be undone.
        </p>
        <div className="row stack-row-actions">
          <button
            ref={cancelRef}
            type="button"
            className="secondary"
            disabled={saving}
            onClick={onCancel}
          >
            Cancel
          </button>
          <button
            type="button"
            className="danger"
            disabled={saving}
            onClick={() => void onConfirm()}
          >
            Delete
          </button>
        </div>
      </section>
    </Modal>
  );
}
