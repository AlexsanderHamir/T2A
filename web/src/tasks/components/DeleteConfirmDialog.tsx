import { useEffect, useRef } from "react";

type Props = {
  taskTitle: string;
  saving: boolean;
  onCancel: () => void;
  onConfirm: () => void;
};

export function DeleteConfirmDialog({
  taskTitle,
  saving,
  onCancel,
  onConfirm,
}: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") onCancel();
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onCancel]);

  return (
    <section
      className="panel confirm-dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="delete-dialog-title"
    >
      <h2 id="delete-dialog-title">Delete task?</h2>
      <p className="muted">
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
  );
}
