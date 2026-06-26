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
        <header className="worktrees-form-modal__header">
          <h2 id="create-branch-title">Add branch</h2>
          <p className="worktrees-form-modal__lead">
            Create a new git branch in this repository.
          </p>
        </header>

        <div className="worktrees-form-modal__body">
          <section className="worktrees-form-modal__section">
            <label className="field">
              <span className="settings-field-label">Branch name</span>
              <input
                type="text"
                value={name}
                required
                disabled={pending}
                placeholder="e.g. feature-auth"
                onChange={(e) => setName(e.target.value)}
              />
            </label>
            <label className="field">
              <span className="settings-field-label">Start point</span>
              <input
                type="text"
                value={startPoint}
                disabled={pending}
                placeholder="e.g. main"
                onChange={(e) => setStartPoint(e.target.value)}
              />
              <span className="worktrees-form-modal__field-hint">
                Commit, tag, or branch to branch from. Defaults to HEAD when empty.
              </span>
            </label>
          </section>
        </div>

        {errorMessage ? (
          <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
        ) : null}

        <footer className="worktrees-form-modal__footer">
          <button type="button" className="secondary" disabled={pending} onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-primary" disabled={pending || !name.trim()}>
            {pending ? "Creating…" : "Create branch"}
          </button>
        </footer>
      </form>
    </Modal>
  );
}
