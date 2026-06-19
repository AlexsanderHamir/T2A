import { Modal } from "@/shared/Modal";
import { shortSha } from "./commitDisplay";
import { CommitDiffView } from "./CommitDiffView";

type Props = {
  sha: string;
  patch: string;
  truncated: boolean;
  onClose: () => void;
};

export function CommitDiffModal({ sha, patch, truncated, onClose }: Props) {
  return (
    <Modal
      onClose={onClose}
      labelledBy="commit-diff-modal-title"
      size="wide"
    >
      <section className="panel modal-sheet commit-diff-modal">
        <header className="commit-diff-modal-head">
          <p className="commit-diff-modal-eyebrow">Commit diff</p>
          <h2 id="commit-diff-modal-title" className="commit-diff-modal-title">
            <code>{shortSha(sha)}</code>
          </h2>
        </header>
        {truncated ? (
          <p className="commit-diff-modal-truncation muted" role="status">
            This patch was truncated at the server limit. The view shows the
            available portion only.
          </p>
        ) : null}
        <div className="commit-diff-modal-body">
          <CommitDiffView patch={patch} className="task-commit-diff-view task-commit-diff-view--modal" />
        </div>
      </section>
    </Modal>
  );
}
