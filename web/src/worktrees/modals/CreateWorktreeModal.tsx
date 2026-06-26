import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { WorkspaceDirPickerModal } from "@/components/workspace-picker";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";
import {
  WorktreeBranchBindFields,
  branchBindPayload,
  type BranchBindValue,
} from "../components/WorktreeBranchBindFields";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  repositoryId: string;
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
  repositoryId,
  onClose,
  onSubmit,
}: Props) {
  const [path, setPath] = useState("");
  const [name, setName] = useState("");
  const [branchBind, setBranchBind] = useState<BranchBindValue>({
    selectedBranchName: "",
    newBranchName: "",
    createNew: false,
  });
  const [pickerOpen, setPickerOpen] = useState(false);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;
  const branchPayload = branchBindPayload(branchBind);
  const canSubmit = path.trim() !== "" && branchPayload != null;

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="create-worktree-title"
        describedBy="create-worktree-lead"
        busy={pending}
        dismissibleWhileBusy={false}
      >
        <form
          className="panel modal-sheet worktrees-form-modal"
          onSubmit={(e) => {
            e.preventDefault();
            const trimmedPath = path.trim();
            if (!trimmedPath || !branchPayload) return;
            onSubmit({
              path: trimmedPath,
              name: name.trim() || undefined,
              branch: branchPayload.name,
              create_branch: branchPayload.create_branch,
            });
          }}
        >
          <header className="worktrees-form-modal__header">
            <h2 id="create-worktree-title">Create worktree</h2>
            <p id="create-worktree-lead" className="worktrees-form-modal__lead">
              Add a new linked worktree directory and choose the checkout branch Hamix registers
              with it.
            </p>
          </header>

          <div className="worktrees-form-modal__body">
            <section
              className="worktrees-form-modal__section"
              aria-labelledby="create-worktree-section-location"
            >
              <h3 id="create-worktree-section-location" className="worktrees-form-modal__section-title">
                Location
              </h3>
              <div className="worktrees-form-modal__picker">
                <p className="worktrees-form-modal__picker-label">Worktree path</p>
                <button
                  type="button"
                  className="secondary"
                  disabled={pending}
                  onClick={() => setPickerOpen(true)}
                >
                  Choose worktree path
                </button>
                {path.trim() !== "" ? (
                  <p className="worktrees-form-modal__selected">
                    Path: <code>{path}</code>
                  </p>
                ) : (
                  <p className="worktrees-form-modal__picker-empty">No folder selected yet.</p>
                )}
              </div>
            </section>

            <section
              className="worktrees-form-modal__section"
              aria-labelledby="create-worktree-section-display"
            >
              <h3 id="create-worktree-section-display" className="worktrees-form-modal__section-title">
                Display
              </h3>
              <label className="field">
                <span className="settings-field-label">Worktree display name</span>
                <input
                  type="text"
                  value={name}
                  disabled={pending}
                  onChange={(e) => setName(e.target.value)}
                  placeholder="e.g. feature-auth"
                />
                <span className="worktrees-form-modal__field-hint">
                  Optional label shown in Hamix instead of the directory path.
                </span>
              </label>
            </section>

            <section
              className="worktrees-form-modal__section"
              aria-labelledby="create-worktree-section-branch"
            >
              <h3 id="create-worktree-section-branch" className="worktrees-form-modal__section-title">
                Branch
              </h3>
              <WorktreeBranchBindFields
                repositoryId={repositoryId}
                enabled={open && repositoryId !== ""}
                pending={pending}
                value={branchBind}
                onChange={setBranchBind}
                branchSelectId="create-worktree-branch-select"
                newBranchInputId="create-worktree-branch-new-name"
                existingBranchLabel="Checkout branch"
                existingBranchHint="Existing repository branch to check out in the new worktree, or create a new branch below."
              />
            </section>
          </div>

          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}

          <footer className="worktrees-form-modal__footer">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              Cancel
            </button>
            <button type="submit" className="btn-primary" disabled={pending || !canSubmit}>
              {pending ? "Creating…" : "Create worktree"}
            </button>
          </footer>
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
