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
        <h2 id="delete-dialog-title">Delete task?</h2>
        <p className="muted" id="delete-dialog-description">
          This cannot be undone. Task: <strong>{taskTitle}</strong>
          {subtaskCount > 0 ? (
            <>
              {" "}
              <br />
              <strong>
                {subtaskCount === 1
                  ? "Its 1 subtask will also be permanently deleted."
                  : `Its ${subtaskCount} subtasks will also be permanently deleted.`}
              </strong>
            </>
          ) : null}
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
