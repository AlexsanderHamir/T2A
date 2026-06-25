import { useState } from "react";
import { Button } from "@/components/ui";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { WorkspaceDirPickerModal } from "@/settings/WorkspaceDirPickerModal";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  onClose: () => void;
  onSubmit: (input: { path: string; default_branch?: string }) => void;
};

export function RegisterRepositoryModal({
  open,
  pending,
  error,
  onClose,
  onSubmit,
}: Props) {
  const [path, setPath] = useState("");
  const [defaultBranch, setDefaultBranch] = useState("main");
  const [pickerOpen, setPickerOpen] = useState(false);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;
  const trimmedPath = path.trim();
  const hasPath = trimmedPath !== "";

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
            if (!hasPath) return;
            onSubmit({
              path: trimmedPath,
              default_branch: defaultBranch.trim() || undefined,
            });
          }}
        >
          <header className="worktrees-form-modal__header">
            <h2 id="register-repo-title">Register repository</h2>
            <p id="register-repo-lead" className="worktrees-form-modal__lead">
              Choose the main checkout path for this project. Hamix registers worktrees and
              branches under it.
            </p>
          </header>

          <div className="worktrees-form-modal__picker">
            <p className="worktrees-form-modal__picker-label" id="register-repo-path-label">
              Repository path
            </p>
            <p
              className="worktrees-form-modal__path-display"
              data-empty={hasPath ? "false" : "true"}
              aria-labelledby="register-repo-path-label"
              aria-live="polite"
            >
              {hasPath ? trimmedPath : "No folder selected yet"}
            </p>
            <Button
              type="button"
              variant="secondary"
              className="worktrees-form-modal__browse-btn"
              disabled={pending}
              onClick={() => setPickerOpen(true)}
            >
              Choose folder
            </Button>
          </div>

          <label className="field">
            <span className="settings-field-label">Default branch</span>
            <input
              type="text"
              value={defaultBranch}
              disabled={pending}
              onChange={(e) => setDefaultBranch(e.target.value)}
              spellCheck={false}
              autoComplete="off"
            />
          </label>

          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}

          <div className="row stack-row-actions">
            <Button type="button" variant="secondary" disabled={pending} onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" loading={pending} disabled={!hasPath}>
              Register
            </Button>
          </div>
        </form>
      </Modal>
      <WorkspaceDirPickerModal
        open={pickerOpen}
        nested
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
