import type { FormEvent } from "react";
import { Modal } from "../../shared/Modal";

type Props = {
  pending: boolean;
  saving: boolean;
  onClose: () => void;
  text: string;
  onTextChange: (v: string) => void;
  onSubmit: (e: FormEvent) => void;
};

export function ChecklistCriterionModal({
  pending,
  saving,
  onClose,
  text,
  onTextChange,
  onSubmit,
}: Props) {
  const disabled = pending || saving;

  return (
    <Modal
      onClose={onClose}
      labelledBy="checklist-criterion-modal-title"
      busy={pending}
      busyLabel="Adding criterion…"
    >
      <section className="panel modal-sheet task-checklist-criterion-modal-sheet">
        <h2 id="checklist-criterion-modal-title">New criterion</h2>
        <p className="muted task-checklist-criterion-modal-lead">
          Add one clear, testable requirement. You can open this again to add
          more.
        </p>
        <form
          className="task-checklist-criterion-modal-form task-create-form"
          onSubmit={onSubmit}
        >
          <div className="field">
            <label htmlFor="checklist-criterion-text">Criterion</label>
            <input
              id="checklist-criterion-text"
              value={text}
              onChange={(ev) => onTextChange(ev.target.value)}
              placeholder="e.g. All subtasks marked done"
              disabled={disabled}
              autoFocus
            />
          </div>
          <div className="row stack-row-actions task-checklist-criterion-modal-actions">
            <button
              type="button"
              className="secondary"
              disabled={disabled}
              onClick={onClose}
            >
              Cancel
            </button>
            <button
              type="submit"
              className="task-create-submit"
              disabled={!text.trim() || disabled}
            >
              Add criterion
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
