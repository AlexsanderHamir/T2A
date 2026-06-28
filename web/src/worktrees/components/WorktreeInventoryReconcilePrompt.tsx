import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { gitReconcileErrorMessage } from "../gitReconcileErrors";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { WorktreeReconcileStatus } from "./WorktreeReconcileStatus";

type Props = {
  storedPath: string;
  pending: boolean;
  reconcileError?: unknown;
  onReconcile: () => void;
};

export function WorktreeInventoryReconcilePrompt({
  storedPath,
  pending,
  reconcileError,
  onReconcile,
}: Props) {
  const reconcileErrorMessage =
    reconcileError != null ? gitReconcileErrorMessage(reconcileError) : null;
  const showStatus = pending || reconcileErrorMessage == null;

  return (
    <div className="worktrees-form-modal__inventory-prompt" role="alert">
      <p className="worktrees-form-modal__inventory-prompt-lead">
        {worktreeGitCopy.liveInventoryReconcileLead}
      </p>
      {storedPath.trim() !== "" ? (
        <p className="worktrees-form-modal__stored-path">
          <span className="worktrees-form-modal__stored-path-label">
            {worktreeGitCopy.relocateModalStoredPathLabel}
          </span>
          <code>{storedPath}</code>
        </p>
      ) : null}
      {showStatus ? <WorktreeReconcileStatus /> : null}
      {reconcileErrorMessage ? (
        <>
          <MutationErrorBanner error={reconcileErrorMessage} className="worktrees-form-modal__error" />
          <button type="button" className="secondary" disabled={pending} onClick={onReconcile}>
            {pending ? worktreeGitCopy.reconciling : worktreeGitCopy.liveInventoryReconcileAction}
          </button>
        </>
      ) : null}
    </div>
  );
}
