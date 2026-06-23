import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import type { GitDeleteTarget } from "../gitDeleteErrors";
import { gitDeleteBlocked, gitDeleteErrorMessage } from "../gitDeleteErrors";

type Props = {
  target: GitDeleteTarget | null;
  pending: boolean;
  error: unknown;
  onClose: () => void;
  onConfirm: () => void;
};

function targetNoun(kind: GitDeleteTarget["kind"]): string {
  switch (kind) {
    case "repository":
      return "repository";
    case "worktree":
      return "worktree";
    case "branch":
      return "branch";
  }
}

export function DeleteConfirmDialog({
  target,
  pending,
  error,
  onClose,
  onConfirm,
}: Props) {
  if (!target) return null;

  const blocked = error != null && gitDeleteBlocked(error);
  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;

  return (
    <Modal
      onClose={onClose}
      labelledBy="git-delete-dialog-title"
      describedBy="git-delete-dialog-description"
      busy={pending}
      dismissibleWhileBusy
    >
      <section className="panel confirm-dialog modal-sheet worktrees-delete-dialog">
        <h2 id="git-delete-dialog-title">Delete {targetNoun(target.kind)}?</h2>
        <p className="confirm-dialog__statement" id="git-delete-dialog-description">
          <strong>{target.label}</strong> will be removed from Hamix and from git where
          applicable.
        </p>
        <p className="confirm-dialog__footnote">This action cannot be undone.</p>
        {errorMessage ? (
          <MutationErrorBanner error={errorMessage} className="confirm-dialog__err" />
        ) : null}
        <div className="row stack-row-actions">
          <button type="button" className="secondary" disabled={pending} onClick={onClose}>
            Cancel
          </button>
          <button
            type="button"
            className="danger"
            disabled={pending || blocked}
            onClick={() => void onConfirm()}
          >
            {pending ? "Deleting…" : "Delete"}
          </button>
        </div>
      </section>
    </Modal>
  );
}
