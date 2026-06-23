import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  onClose: () => void;
  onSubmit: (input: { name: string; start_point?: string }) => void;
};

export function CreateBranchModal({ open, pending, error, onClose, onSubmit }: Props) {
  const [name, setName] = useState("");
  const [startPoint, setStartPoint] = useState("");

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;

  return (
    <Modal
      onClose={onClose}
      labelledBy="create-branch-title"
      busy={pending}
      dismissibleWhileBusy={false}
    >
      <form
        className="panel modal-sheet worktrees-form-modal"
        onSubmit={(e) => {
          e.preventDefault();
          const trimmed = name.trim();
          if (!trimmed) return;
          onSubmit({
            name: trimmed,
            start_point: startPoint.trim() || undefined,
          });
        }}
      >
        <h2 id="create-branch-title">Add branch</h2>
        <label className="field">
          <span className="settings-field-label">Branch name</span>
          <input
            type="text"
            value={name}
            required
            disabled={pending}
            onChange={(e) => setName(e.target.value)}
          />
        </label>
        <label className="field">
          <span className="settings-field-label">Start point</span>
          <input
            type="text"
            value={startPoint}
            disabled={pending}
            placeholder="Optional (defaults to HEAD)"
            onChange={(e) => setStartPoint(e.target.value)}
          />
        </label>
        {errorMessage ? (
          <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
        ) : null}
        <div className="row stack-row-actions">
          <button type="button" className="secondary" disabled={pending} onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={pending || !name.trim()}>
            {pending ? "Creating…" : "Create branch"}
          </button>
        </div>
      </form>
    </Modal>
  );
}
