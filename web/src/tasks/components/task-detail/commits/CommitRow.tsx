import { Link } from "react-router-dom";
import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useNow } from "@/shared/useNow";
import { usePrefetchOnIntent, useCommitDiffPrefetcher } from "@/app/hooks/usePrefetchOnIntent";
import type { CycleCommit, TaskCommit } from "@/types";
import { shortSha, taskCommitDiffPath } from "./commitDisplay";

function attemptSeqForRow(commit: CycleCommit): number | undefined {
  return "attempt_seq" in commit
    ? (commit as TaskCommit).attempt_seq
    : undefined;
}

type Props = {
  taskId: string;
  commit: CycleCommit;
  showAttempt?: boolean;
};

export function CommitRow({ taskId, commit, showAttempt = false }: Props) {
  const now = useNow();
  const prefetchCommitDiff = useCommitDiffPrefetcher();
  const diffIntent = usePrefetchOnIntent(() => prefetchCommitDiff(commit.sha));
  const attemptSeq = attemptSeqForRow(commit);
  const diffTo = taskCommitDiffPath(taskId, commit.sha);
  const ariaLabel = `View diff for ${shortSha(commit.sha)}: ${commit.message}`;

  return (
    <li className="task-commit-row">
      <Link
        to={diffTo}
        className="task-commit-row-link"
        aria-label={ariaLabel}
        {...diffIntent}
      >
        <span className="task-commit-row-inner">
          <code className="task-commit-sha" title={commit.sha}>
            {shortSha(commit.sha)}
          </code>
          <span className="task-commit-message">{commit.message}</span>
          <span className="task-commit-meta muted">
            {showAttempt && attemptSeq != null ? (
              <>
                Attempt #{attemptSeq}
                <span className="task-commit-meta-sep" aria-hidden="true">
                  ·
                </span>
              </>
            ) : null}
            {formatRelativeTime(commit.committed_at, new Date(now))}
          </span>
        </span>
      </Link>
    </li>
  );
}
