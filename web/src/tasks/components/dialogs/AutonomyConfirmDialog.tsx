import { useEffect, useRef } from "react";
import { Modal } from "../../../shared/Modal";
import { MutationErrorBanner } from "../../../shared/MutationErrorBanner";

/**
 * `enable` is true when the operator is moving the task from `on_hold`
 * to `ready` (handing control back to the agent), false when they are
 * moving from `ready` to `on_hold` (parking the task). The two
 * directions warrant different copy and a different action button
 * label, but the same dialog chrome.
 */
type Props = {
  enable: boolean;
  taskTitle: string;
  saving: boolean;
  pending: boolean;
  error?: string | null;
  onCancel: () => void;
  onConfirm: () => void;
};

export function AutonomyConfirmDialog({
  enable,
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

  const title = enable
    ? "Resume autonomous execution?"
    : "Put this task on hold?";
  const body = enable
    ? "The agent will pick this task up as soon as scheduling and dependencies allow."
    : "The agent will stop considering this task. You can resume it any time from this page.";
  const confirmLabel = enable ? "Resume" : "Put on hold";

  return (
    <Modal
      onClose={onCancel}
      labelledBy="autonomy-dialog-title"
      describedBy="autonomy-dialog-description"
      busy={pending}
      dismissibleWhileBusy
    >
      <section className="panel confirm-dialog modal-sheet">
        <h2 id="autonomy-dialog-title">{title}</h2>
        <p
          className="confirm-dialog__statement"
          id="autonomy-dialog-description"
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
            className="primary"
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
