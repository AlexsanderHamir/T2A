import { useEffect, useRef } from "react";
import { Modal } from "../../../shared/Modal";
import { MutationErrorBanner } from "../../../shared/MutationErrorBanner";

export type TaskRetryMode = "fresh" | "resume";

type Props = {
  mode: TaskRetryMode;
  taskTitle: string;
  saving: boolean;
  pending: boolean;
  error?: string | null;
  onCancel: () => void;
  onConfirm: () => void;
};

export function TaskRetryConfirmDialog({
  mode,
  taskTitle,
  saving,
  pending,
  error = null,
  onCancel,
  onConfirm,
}: Props) {
  const cancelRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    cancelRef.current?.focus();
  }, []);

  const isFresh = mode === "fresh";
  const title = isFresh ? "Start over?" : "Resume from failure?";
  const body = isFresh
    ? "This discards this attempt's git changes and untracked files in the repo, then queues a new run from a clean tree."
    : "This starts a new attempt and continues from the failed attempt's checkpoint. Git history is kept.";
  const confirmLabel = isFresh ? "Start over" : "Resume from failure";

  return (
    <Modal
      onClose={onCancel}
      labelledBy="task-retry-dialog-title"
      describedBy="task-retry-dialog-description"
      busy={pending}
      dismissibleWhileBusy
    >
      <section className="panel confirm-dialog modal-sheet">
        <h2 id="task-retry-dialog-title">{title}</h2>
        <p
          className="confirm-dialog__statement"
          id="task-retry-dialog-description"
        >
          <strong>{taskTitle}</strong>
        </p>
        <p className="confirm-dialog__footnote">{body}</p>
        <MutationErrorBanner error={error} className="confirm-dialog__err" />
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
            className={isFresh ? "primary" : "secondary"}
            disabled={saving}
            onClick={() => void onConfirm()}
          >
            {confirmLabel}
          </button>
        </div>
      </section>
    </Modal>
  );
}
