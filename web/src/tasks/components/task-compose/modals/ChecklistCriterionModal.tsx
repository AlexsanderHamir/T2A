import type { FormEvent } from "react";
import { FieldLabel } from "@/shared/FieldLabel";
import { Modal } from "../../../../shared/Modal";

type Props = {
  mode: "add" | "edit";
  pending: boolean;
  saving: boolean;
  onClose: () => void;
  text: string;
  onTextChange: (v: string) => void;
  onSubmit: (e: FormEvent) => void;
  /** When opened above another dialog (e.g. new-task modal). */
  modalStack?: "default" | "nested";
  lockBodyScroll?: boolean;
  /**
   * Allow Escape / backdrop click to dismiss the modal even while the
   * underlying mutation is `pending`. Caller is responsible for
   * ensuring the mutation's settle handler is race-hardened against a
   * stale resolution after dismiss (see
   * `.agent/frontend-improvement-agent.log` Session 30 for the
   * `useTaskDetailChecklist` add/edit flows). Default `false`
   * preserves the legacy "busy locks close" contract for the
   * `TaskComposeFields` caller, where the modal manages local state
   * only and `pending` is permanently false anyway.
   */
  dismissibleWhileBusy?: boolean;
};

export function ChecklistCriterionModal({
  mode,
  pending,
  saving,
  onClose,
  text,
  onTextChange,
  onSubmit,
  modalStack = "default",
  lockBodyScroll = true,
  dismissibleWhileBusy = false,
}: Props) {
  const disabled = pending || saving;
  const titleId =
    mode === "add"
      ? "checklist-criterion-modal-title"
      : "checklist-criterion-edit-modal-title";
  const busyLabel =
    mode === "add" ? "Adding criterion…" : "Saving changes…";

  return (
    <Modal
      onClose={onClose}
      labelledBy={titleId}
      describedBy="checklist-criterion-modal-description"
      busy={pending}
      busyLabel={busyLabel}
      stack={modalStack}
      lockBodyScroll={lockBodyScroll}
      dismissibleWhileBusy={dismissibleWhileBusy}
    >
      <section className="panel modal-sheet task-checklist-criterion-modal-sheet">
        <h2 id={titleId}>
          {mode === "add" ? "New criterion" : "Edit criterion"}
        </h2>
        <p
          className="muted task-checklist-criterion-modal-lead"
          id="checklist-criterion-modal-description"
        >
          {mode === "add" ? (
            <>
              Add one clear, testable requirement. You can open this again to
              add more.
            </>
          ) : (
            <>Update the wording for this requirement.</>
          )}
        </p>
        <form
          className="task-checklist-criterion-modal-form task-create-form"
          onSubmit={(e) => {
            e.stopPropagation();
            onSubmit(e);
          }}
        >
          <div className="field">
            <FieldLabel htmlFor="checklist-criterion-text" requirement="required">
              Criterion
            </FieldLabel>
            <input
              id="checklist-criterion-text"
              value={text}
              onChange={(ev) => onTextChange(ev.target.value)}
              placeholder="e.g. All subtasks marked done"
              disabled={disabled}
              autoFocus
              required
              aria-required="true"
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
              {mode === "add" ? "Add criterion" : "Save changes"}
            </button>
          </div>
        </form>
      </section>
    </Modal>
  );
}
