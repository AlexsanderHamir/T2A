import { useEffect, useMemo, useState } from "react";
import { Button } from "@/components/ui";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { WorkspaceDirPickerModal } from "@/settings/WorkspaceDirPickerModal";
import { CustomSelect } from "@/tasks/components/custom-select/CustomSelect";
import { useGitRepositoryProbe } from "../hooks/useGitRepositoryProbe";
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
  const [defaultBranch, setDefaultBranch] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);

  const trimmedPath = path.trim();
  const hasPath = trimmedPath !== "";

  const probeQuery = useGitRepositoryProbe(trimmedPath, {
    enabled: open && hasPath,
  });
  const probe = probeQuery.data;
  const isGitRepo = probe?.is_git_repository === true;
  const branches = probe?.branches ?? [];

  const branchOptions = useMemo(
    () =>
      branches.map((b) => ({
        value: b.name,
        label:
          b.name === probe?.current_branch ? `${b.name} (current)` : b.name,
      })),
    [branches, probe?.current_branch],
  );

  useEffect(() => {
    if (!open) {
      setPath("");
      setDefaultBranch("");
      setPickerOpen(false);
      return;
    }
    if (!hasPath) {
      setDefaultBranch("");
      return;
    }
    if (!isGitRepo || branches.length === 0) {
      setDefaultBranch("");
      return;
    }
    const preferred =
      probe?.current_branch &&
      branches.some((b) => b.name === probe.current_branch)
        ? probe.current_branch
        : branches[0]?.name ?? "";
    setDefaultBranch((prev) =>
      prev !== "" && branches.some((b) => b.name === prev) ? prev : preferred,
    );
  }, [open, hasPath, isGitRepo, branches, probe?.current_branch]);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;
  const branchesLoading = hasPath && (probeQuery.isLoading || probeQuery.isFetching);
  const canRegister =
    hasPath && isGitRepo && defaultBranch.trim() !== "" && !branchesLoading && !pending;

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
            if (!canRegister) return;
            onSubmit({
              path: trimmedPath,
              default_branch: defaultBranch.trim(),
            });
          }}
        >
          <header className="worktrees-form-modal__header">
            <h2 id="register-repo-title">Register repository</h2>
            <p id="register-repo-lead" className="worktrees-form-modal__lead">
              Choose a folder that contains a git checkout. Hamix registers worktrees and
              branches under it.
            </p>
          </header>

          <div className="worktrees-form-modal__picker">
            <div className="worktrees-form-modal__picker-main">
              <span
                className={
                  isGitRepo
                    ? "worktrees-form-modal__picker-icon worktrees-form-modal__picker-icon--git"
                    : "worktrees-form-modal__picker-icon"
                }
                aria-hidden="true"
              >
                {isGitRepo ? <GitBranchGlyph /> : <FolderGlyph />}
              </span>
              <span className="worktrees-form-modal__picker-text">
                <span
                  className="worktrees-form-modal__picker-label"
                  id="register-repo-path-label"
                >
                  Repository path
                </span>
                {hasPath ? (
                  <code
                    className="worktrees-form-modal__path-display"
                    aria-labelledby="register-repo-path-label"
                  >
                    {trimmedPath}
                  </code>
                ) : (
                  <span
                    className="worktrees-form-modal__path-display worktrees-form-modal__path-display--empty"
                    aria-labelledby="register-repo-path-label"
                  >
                    No folder selected yet
                  </span>
                )}
              </span>
              <Button
                type="button"
                variant="secondary"
                className="worktrees-form-modal__browse-btn"
                disabled={pending}
                onClick={() => setPickerOpen(true)}
              >
                {hasPath ? "Change" : "Choose folder"}
              </Button>
            </div>
            {hasPath && probeQuery.isSuccess && !isGitRepo ? (
              <p
                className="worktrees-form-modal__git-status worktrees-form-modal__git-status--missing"
                role="status"
              >
                This path is not a git checkout — choose a folder with a .git directory
              </p>
            ) : null}
          </div>

          {hasPath && isGitRepo ? (
            <>
              {branchesLoading ? (
                <p className="settings-field-help" role="status">
                  Loading branches…
                </p>
              ) : branches.length === 0 ? (
                <p className="settings-field-help" role="status">
                  No branches found in this repository.
                </p>
              ) : (
                <CustomSelect
                  id="register-repo-default-branch"
                  label="Default branch"
                  value={defaultBranch}
                  options={branchOptions}
                  disabled={pending}
                  requirement="required"
                  onChange={setDefaultBranch}
                />
              )}
              <p className="settings-field-help">
                Pick an existing branch from this checkout. Hamix stores it as the repository
                default — registration does not create a new branch.
              </p>
            </>
          ) : null}

          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}

          <div className="row stack-row-actions">
            <Button type="button" variant="secondary" disabled={pending} onClick={onClose}>
              Cancel
            </Button>
            <Button type="submit" variant="primary" loading={pending} disabled={!canRegister}>
              Register
            </Button>
          </div>
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

function FolderGlyph() {
  return (
    <svg viewBox="0 0 20 20" width="20" height="20" aria-hidden="true">
      <path
        d="M2.75 5.5A1.75 1.75 0 0 1 4.5 3.75h3.13c.46 0 .9.18 1.23.5l1.12 1.06H15.5c.97 0 1.75.78 1.75 1.75v7c0 .97-.78 1.75-1.75 1.75h-11A1.75 1.75 0 0 1 2.75 14V5.5Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function GitBranchGlyph() {
  return (
    <svg viewBox="0 0 20 20" width="20" height="20" aria-hidden="true">
      <path
        d="M6 4.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3ZM6 12.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3ZM14 4.5a1.5 1.5 0 1 0 0 3 1.5 1.5 0 0 0 0-3Z"
        fill="currentColor"
      />
      <path
        d="M6 7.5v5M14 7.5a4 4 0 0 1-4 4H6"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
