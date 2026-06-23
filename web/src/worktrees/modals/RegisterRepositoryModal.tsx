import { useState } from "react";
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

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="register-repo-title"
        busy={pending}
        dismissibleWhileBusy={false}
      >
        <form
          className="panel modal-sheet worktrees-form-modal"
          onSubmit={(e) => {
            e.preventDefault();
            const trimmed = path.trim();
            if (!trimmed) return;
            onSubmit({
              path: trimmed,
              default_branch: defaultBranch.trim() || undefined,
            });
          }}
        >
          <h2 id="register-repo-title">Register repository</h2>
          <p className="worktrees-form-modal__lead">
            Choose the main checkout path for this project. Hamix registers worktrees and
            branches under it.
          </p>
          <div className="worktrees-form-modal__picker">
            <button
              type="button"
              className="btn-primary"
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
              <p className="settings-field-help">No folder selected yet.</p>
            )}
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
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={pending || !path.trim()}>
              {pending ? "Registering…" : "Register"}
            </button>
          </div>
        </form>
      </Modal>
      <WorkspaceDirPickerModal
        open={pickerOpen}
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
