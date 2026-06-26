import { useEffect, useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { CustomSelect } from "@/components/custom-select";
import { useGlobalLiveWorktrees } from "../hooks/useGlobalLiveWorktrees";
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
    branch?: { name: string; create_branch?: boolean };
  }) => void;
};

export function RegisterWorktreeModal({
  open,
  pending,
  error,
  repositoryId,
  onClose,
  onSubmit,
}: Props) {
  const [selectedPath, setSelectedPath] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [branchBind, setBranchBind] = useState<BranchBindValue>({
    selectedBranchName: "",
    newBranchName: "",
    createNew: false,
  });

  const liveWorktreesQuery = useGlobalLiveWorktrees(repositoryId, {
    enabled: open && repositoryId !== "",
  });
  const liveWorktrees = (liveWorktreesQuery.data ?? []).filter((wt) => !wt.registered);
  const worktreeOptions = liveWorktrees.map((wt) => ({
    value: wt.path,
    label: wt.is_main ? `${wt.path} (main)` : wt.path,
  }));

  useEffect(() => {
    if (!open || selectedPath === "" || branchBind.createNew) return;
    const match = liveWorktreesQuery.data?.find((wt) => wt.path === selectedPath);
    const branch = match?.branch.trim() ?? "";
    if (branch === "") return;
    setBranchBind((prev) =>
      prev.selectedBranchName === branch ? prev : { ...prev, selectedBranchName: branch },
    );
  }, [open, selectedPath, branchBind.createNew, liveWorktreesQuery.data]);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;
  const branchPayload = branchBindPayload(branchBind);
  const canSubmit = selectedPath !== "" && branchPayload != null;

  return (
    <Modal
      onClose={onClose}
      labelledBy="register-worktree-title"
      describedBy="register-worktree-lead"
      busy={pending}
      dismissibleWhileBusy={false}
    >
      <form
        className="panel modal-sheet worktrees-form-modal"
        onSubmit={(e) => {
          e.preventDefault();
          if (!canSubmit || !branchPayload) return;
          onSubmit({
            path: selectedPath,
            name: displayName.trim() || undefined,
            branch: branchPayload,
          });
        }}
      >
        <header className="worktrees-form-modal__header">
          <h2 id="register-worktree-title">Register worktree</h2>
          <p id="register-worktree-lead" className="worktrees-form-modal__lead">
            Choose a linked worktree directory and the branch to register with it.
          </p>
        </header>

        <div className="worktrees-form-modal__body">
          <section
            className="worktrees-form-modal__section"
            aria-labelledby="register-worktree-section-location"
          >
            <h3 id="register-worktree-section-location" className="worktrees-form-modal__section-title">
              Location
            </h3>
            <CustomSelect
              id="register-worktree-select"
              label="Worktree path"
              value={selectedPath}
              options={worktreeOptions}
              disabled={pending || liveWorktreesQuery.isLoading || worktreeOptions.length === 0}
              requirement="required"
              onChange={setSelectedPath}
            />
            {worktreeOptions.length === 0 && !liveWorktreesQuery.isLoading ? (
              <p className="worktrees-form-modal__callout">
                No unregistered linked worktrees found. Use Create worktree or run git worktree add
                outside Hamix first.
              </p>
            ) : null}
          </section>

          <section
            className="worktrees-form-modal__section"
            aria-labelledby="register-worktree-section-display"
          >
            <h3 id="register-worktree-section-display" className="worktrees-form-modal__section-title">
              Display
            </h3>
            <label className="field">
              <span className="settings-field-label">Worktree display name</span>
              <input
                type="text"
                value={displayName}
                disabled={pending}
                placeholder="e.g. feature-auth"
                onChange={(e) => setDisplayName(e.target.value)}
              />
              <span className="worktrees-form-modal__field-hint">
                Optional label shown in Hamix instead of the directory path.
              </span>
            </label>
          </section>

          <section
            className="worktrees-form-modal__section"
            aria-labelledby="register-worktree-section-branch"
          >
            <h3 id="register-worktree-section-branch" className="worktrees-form-modal__section-title">
              Branch
            </h3>
            <WorktreeBranchBindFields
              repositoryId={repositoryId}
              enabled={open && repositoryId !== ""}
              pending={pending}
              value={branchBind}
              onChange={setBranchBind}
              branchSelectId="register-worktree-branch-select"
              existingBranchLabel="Existing repository branch"
              existingBranchHint="Pick a branch already in this repository to associate with the worktree. Defaults to the worktree's current checkout when available."
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
            {pending ? "Registering…" : "Register worktree"}
          </button>
        </footer>
      </form>
    </Modal>
  );
}
