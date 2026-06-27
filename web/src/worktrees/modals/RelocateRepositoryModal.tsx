import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { WorkspaceDirPickerModal } from "@/components/workspace-picker";
import { gitReconcileErrorMessage } from "../gitReconcileErrors";
import { parentBrowsePath } from "../parentBrowsePath";
import { worktreeGitCopy } from "../worktreeGitCopy";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  storedPath: string;
  onClose: () => void;
  onSubmit: (input: { path: string }) => void;
};

export function RelocateRepositoryModal({
  open,
  pending,
  error,
  storedPath,
  onClose,
  onSubmit,
}: Props) {
  const [path, setPath] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);

  if (!open) return null;

  const errorMessage = error != null ? gitReconcileErrorMessage(error) : null;
  const browseParent = parentBrowsePath(storedPath);

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="relocate-repo-title"
        describedBy="relocate-repo-lead"
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
            <h2 id="relocate-repo-title">{worktreeGitCopy.relocateModalTitle}</h2>
            <p id="relocate-repo-lead" className="worktrees-form-modal__lead">
              {worktreeGitCopy.relocateModalLead}
            </p>
            {storedPath.trim() !== "" ? (
              <p className="worktrees-form-modal__stored-path">
                <span className="worktrees-form-modal__stored-path-label">
                  {worktreeGitCopy.relocateModalStoredPathLabel}
                </span>
                <code>{storedPath}</code>
              </p>
            ) : null}
          </header>
          <div className="worktrees-form-modal__picker">
            <p className="worktrees-form-modal__picker-label">
              {worktreeGitCopy.relocateModalPathLabel}
            </p>
            <button
              type="button"
              className="secondary"
              disabled={pending}
              onClick={() => setPickerOpen(true)}
            >
              {worktreeGitCopy.relocateModalChoosePath}
            </button>
            {path.trim() !== "" ? (
              <p className="worktrees-form-modal__selected">
                {worktreeGitCopy.relocateModalSelectedPrefix}{" "}
                <code>{path}</code>
              </p>
            ) : (
              <p className="worktrees-form-modal__picker-empty">
                {worktreeGitCopy.relocateModalNoPath}
              </p>
            )}
          </div>
          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}
          <div className="row stack-row-actions">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              {worktreeGitCopy.cancel}
            </button>
            <button type="submit" className="btn-primary" disabled={pending || !path.trim()}>
              {pending ? worktreeGitCopy.relocateModalSubmitting : worktreeGitCopy.relocateModalSubmit}
            </button>
          </div>
        </form>
      </Modal>
      <WorkspaceDirPickerModal
        open={pickerOpen}
        nested
        requireGitRepository
        currentPath={path}
        initialBrowsePath={browseParent !== "" ? browseParent : undefined}
        onClose={() => setPickerOpen(false)}
        onSelect={(next) => {
          setPath(next);
          setPickerOpen(false);
        }}
      />
    </>
  );
}
