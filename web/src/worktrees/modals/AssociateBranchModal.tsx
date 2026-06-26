import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { CustomSelect } from "@/components/custom-select";
import { useGlobalLiveBranches } from "../hooks/useGlobalBranches";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  repositoryId: string;
  onClose: () => void;
  onSubmit: (input: { branch_id?: string; name?: string; create_branch?: boolean }) => void;
};

export function AssociateBranchModal({
  open,
  pending,
  error,
  repositoryId,
  onClose,
  onSubmit,
}: Props) {
  const [selectedBranchName, setSelectedBranchName] = useState("");
  const [newBranchName, setNewBranchName] = useState("");
  const [createNew, setCreateNew] = useState(false);

  const liveBranchesQuery = useGlobalLiveBranches(repositoryId, {
    enabled: open && repositoryId !== "",
  });
  const liveBranches = liveBranchesQuery.data ?? [];

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;

  const branchOptions = liveBranches.map((b) => ({ value: b.name, label: b.name }));

  const canSubmit = createNew ? newBranchName.trim() !== "" : selectedBranchName !== "";

  return (
    <Modal
      onClose={onClose}
      labelledBy="associate-branch-title"
      describedBy="associate-branch-lead"
      busy={pending}
      dismissibleWhileBusy={false}
    >
      <form
        className="panel modal-sheet worktrees-form-modal"
        onSubmit={(e) => {
          e.preventDefault();
          if (!canSubmit) return;
          if (createNew) {
            onSubmit({ name: newBranchName.trim(), create_branch: true });
          } else {
            onSubmit({ name: selectedBranchName });
          }
        }}
      >
        <header className="worktrees-form-modal__header">
          <h2 id="associate-branch-title">Add branch</h2>
          <p id="associate-branch-lead" className="worktrees-form-modal__lead">
            Link another repository branch to this worktree for agent runs.
          </p>
        </header>

        <div className="worktrees-form-modal__body">
          <section
            className="worktrees-form-modal__section"
            aria-labelledby="associate-branch-section-branch"
          >
            <h3 id="associate-branch-section-branch" className="worktrees-form-modal__section-title">
              Branch
            </h3>

            <label className="worktrees-form-modal__checkbox">
              <input
                type="checkbox"
                checked={createNew}
                disabled={pending}
                onChange={(e) => setCreateNew(e.target.checked)}
              />
              Create a new branch
            </label>

            {createNew ? (
              <label className="field">
                <span className="settings-field-label">New branch name</span>
                <input
                  type="text"
                  value={newBranchName}
                  required
                  disabled={pending}
                  placeholder="e.g. feature-auth"
                  onChange={(e) => setNewBranchName(e.target.value)}
                />
              </label>
            ) : (
              <div className="worktrees-form-modal__field-group">
                <CustomSelect
                  id="associate-branch-select"
                  label="Existing repository branch"
                  value={selectedBranchName}
                  options={branchOptions}
                  disabled={pending || liveBranchesQuery.isLoading || branchOptions.length === 0}
                  requirement="required"
                  onChange={setSelectedBranchName}
                />
                <p className="worktrees-form-modal__field-hint">
                  Associate another branch from this repository with the worktree.
                </p>
              </div>
            )}
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
            {pending ? "Adding…" : "Add branch"}
          </button>
        </footer>
      </form>
    </Modal>
  );
}
