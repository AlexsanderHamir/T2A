import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { WorkspaceDirPickerModal } from "@/components/workspace-picker";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  onClose: () => void;
  onSubmit: (input: { path: string }) => void;
};

export function RegisterRepositoryModal({
  open,
  pending,
  error,
  onClose,
  onSubmit,
}: Props) {
  const [path, setPath] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="register-repo-title"
        describedBy="register-repo-lead"
        busy={pending}
        dismissibleWhileBusy={false}
      >
        <form
          className="panel modal-sheet worktrees-form-modal"
          onSubmit={(e) => {
            e.preventDefault();
            const trimmed = path.trim();
            if (!trimmed) return;
            onSubmit({ path: trimmed });
          }}
        >
          <header className="worktrees-form-modal__header">
            <h2 id="register-repo-title">Register repository</h2>
            <p id="register-repo-lead" className="worktrees-form-modal__lead">
              Choose the main git checkout on disk. After registering, add worktrees and bind
              branches from the repository card.
            </p>
          </header>
          <div className="worktrees-form-modal__body">
            <section
              className="worktrees-form-modal__section"
              aria-labelledby="register-repo-section-location"
            >
              <h3 id="register-repo-section-location" className="worktrees-form-modal__section-title">
                Location
              </h3>
              <div className="worktrees-form-modal__picker">
                <p className="worktrees-form-modal__picker-label">Repository path</p>
                <button
                  type="button"
                  className="secondary"
                  disabled={pending}
                  onClick={() => setPickerOpen(true)}
                >
                  Choose folder
                </button>
                {path.trim() !== "" ? (
                  <p className="worktrees-form-modal__selected">
                    Selected: <code>{path}</code>
                  </p>
                ) : (
                  <p className="worktrees-form-modal__picker-empty">No folder selected yet.</p>
                )}
              </div>
            </section>
          </div>
          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}
          <footer className="worktrees-form-modal__footer">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={pending || !path.trim()}>
              {pending ? "Registering…" : "Register"}
            </button>
          </footer>
        </form>
      </Modal>
      <WorkspaceDirPickerModal
        open={pickerOpen}
        nested
        requireGitRepository
        currentPath={path}
        onClose={() => setPickerOpen(false)}
        onSelect={(next) => {
          setPath(next);
          setPickerOpen(false);
        }}
      />
    </>
  );
}
