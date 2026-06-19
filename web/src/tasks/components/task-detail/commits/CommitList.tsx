import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useNow } from "@/shared/useNow";
import type { TaskCommit } from "@/types";
import { CommitStatusBadge } from "./CommitStatusBadge";
import { shortSha } from "./commitDisplay";

type Props = {
  commits: ReadonlyArray<TaskCommit>;
  /** When true, show attempt number in each row (task-wide panel). */
  showAttempt?: boolean;
};

export function CommitList({ commits, showAttempt = false }: Props) {
  const now = useNow();

  return (
    <ul className="task-commits-list" data-testid="task-commits-list">
      {commits.map((commit) => (
        <li key={commit.sha} className="task-commit-row">
          <div className="task-commit-row-inner">
            <CommitStatusBadge
              status={commit.status}
              gateReason={commit.gate_reason}
            />
            <code className="task-commit-sha" title={commit.sha}>
              {shortSha(commit.sha)}
            </code>
            <span className="task-commit-message">{commit.message}</span>
            <span className="task-commit-meta muted">
              {showAttempt ? (
                <>
                  Attempt #{commit.attempt_seq}
                  <span className="task-commit-meta-sep" aria-hidden="true">
                    ·
                  </span>
                </>
              ) : null}
              {formatRelativeTime(commit.committed_at, new Date(now))}
            </span>
          </div>
        </li>
      ))}
    </ul>
  );
}
