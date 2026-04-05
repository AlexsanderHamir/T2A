import { useEffect, useRef } from "react";
import { Modal } from "../../shared/Modal";

type Props = {
  taskTitle: string;
  saving: boolean;
  deletePending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function DeleteConfirmDialog({
  taskTitle,
  saving,
  deletePending,
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
    >
      <section className="panel confirm-dialog modal-sheet">
        <h2 id="delete-dialog-title">Delete task?</h2>
        <p className="muted" id="delete-dialog-description">
          This cannot be undone. Task: <strong>{taskTitle}</strong>
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
