import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { WorkspaceDirPickerModal } from "@/settings/WorkspaceDirPickerModal";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  defaultBranch?: string;
  onClose: () => void;
  onSubmit: (input: {
    path: string;
    name?: string;
    branch: string;
    create_branch?: boolean;
  }) => void;
};

export function CreateWorktreeModal({
  open,
  pending,
  error,
  defaultBranch = "main",
  onClose,
  onSubmit,
}: Props) {
  const [path, setPath] = useState("");
  const [name, setName] = useState("");
  const [branch, setBranch] = useState(defaultBranch);
  const [createBranch, setCreateBranch] = useState(false);
  const [pickerOpen, setPickerOpen] = useState(false);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="create-worktree-title"
        busy={pending}
        dismissibleWhileBusy={false}
      >
        <form
          className="panel modal-sheet worktrees-form-modal"
          onSubmit={(e) => {
            e.preventDefault();
            const trimmedPath = path.trim();
            const trimmedBranch = branch.trim();
            if (!trimmedPath || !trimmedBranch) return;
            onSubmit({
              path: trimmedPath,
              name: name.trim() || undefined,
              branch: trimmedBranch,
              create_branch: createBranch,
            });
          }}
        >
          <h2 id="create-worktree-title">Add worktree</h2>
          <div className="worktrees-form-modal__picker">
            <button
              type="button"
              className="btn-primary"
              disabled={pending}
              onClick={() => setPickerOpen(true)}
            >
              Choose worktree path
            </button>
            {path.trim() !== "" ? (
              <p className="worktrees-form-modal__selected">
                Path: <code>{path}</code>
              </p>
            ) : null}
          </div>
          <label className="field">
            <span className="settings-field-label">Display name</span>
            <input
              type="text"
              value={name}
              disabled={pending}
              onChange={(e) => setName(e.target.value)}
              placeholder="Optional"
            />
          </label>
          <label className="field">
            <span className="settings-field-label">Branch</span>
            <input
              type="text"
              value={branch}
              required
              disabled={pending}
              onChange={(e) => setBranch(e.target.value)}
            />
          </label>
          <label className="worktrees-form-modal__checkbox">
            <input
              type="checkbox"
              checked={createBranch}
              disabled={pending}
              onChange={(e) => setCreateBranch(e.target.checked)}
            />
            Create branch if it does not exist
          </label>
          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}
          <div className="row stack-row-actions">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              Cancel
            </button>
            <button
              type="submit"
              className="btn-primary"
              disabled={pending || !path.trim() || !branch.trim()}
            >
              {pending ? "Creating…" : "Create worktree"}
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
