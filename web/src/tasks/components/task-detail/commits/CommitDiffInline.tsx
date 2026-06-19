import { useState } from "react";
import { errorMessage } from "@/lib/errorMessage";
import { useCommitDiff } from "@/tasks/hooks/useCommitDiff";
import { CommitDiffModal } from "./CommitDiffModal";
import { CommitDiffView, countDiffFiles } from "./CommitDiffView";

const maxInlineFiles = 3;

type Props = {
  sha: string;
  enabled: boolean;
};

export function CommitDiffInline({ sha, enabled }: Props) {
  const diffQuery = useCommitDiff(sha, { enabled });
  const [modalOpen, setModalOpen] = useState(false);

  if (!enabled) {
    return null;
  }

  if (diffQuery.isPending) {
    return (
      <div
        className="task-commit-diff-inline task-commit-diff-inline--loading"
        aria-busy="true"
        aria-label="Loading commit diff"
      >
        <div className="task-commit-diff-skeleton" />
        <div className="task-commit-diff-skeleton" />
      </div>
    );
  }

  if (diffQuery.isError) {
    return (
      <div className="task-commit-diff-inline" role="alert">
        <p className="task-commit-diff-error">
          {errorMessage(diffQuery.error, "Could not load diff.")}
        </p>
        <button
          type="button"
          className="secondary task-commit-diff-retry"
          onClick={() => {
            void diffQuery.refetch();
          }}
        >
          Try again
        </button>
      </div>
    );
  }

  if (diffQuery.data === null) {
    return (
      <div className="task-commit-diff-inline">
        <p className="task-commit-diff-empty muted">
          Workspace repo is not configured. Set repo root in Settings to view diffs.
        </p>
      </div>
    );
  }

  const { patch, truncated } = diffQuery.data;
  const fileCount = countDiffFiles(patch);
  const showFullLink = truncated || fileCount > maxInlineFiles;

  return (
    <div className="task-commit-diff-inline">
      {truncated ? (
        <p className="task-commit-diff-truncation muted" role="status">
          Diff preview is truncated. Open the full diff to see the complete patch.
        </p>
      ) : null}
      <CommitDiffView patch={patch} />
      {showFullLink ? (
        <button
          type="button"
          className="secondary task-commit-diff-full-btn"
          onClick={() => setModalOpen(true)}
        >
          View full diff
        </button>
      ) : null}
      {modalOpen ? (
        <CommitDiffModal
          sha={sha}
          patch={patch}
          truncated={truncated}
          onClose={() => setModalOpen(false)}
        />
      ) : null}
    </div>
  );
}
