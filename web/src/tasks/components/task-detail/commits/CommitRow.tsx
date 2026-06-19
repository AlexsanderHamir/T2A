import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useNow } from "@/shared/useNow";
import type { CycleCommit, TaskCommit } from "@/types";
import { CommitStatusBadge } from "./CommitStatusBadge";
import { CommitDiffInline } from "./CommitDiffInline";
import { shortSha } from "./commitDisplay";

function attemptSeqForRow(commit: CycleCommit): number | undefined {
  return "attempt_seq" in commit
    ? (commit as TaskCommit).attempt_seq
    : undefined;
}

type Props = {
  commit: CycleCommit;
  showAttempt?: boolean;
  open: boolean;
  onToggle: (sha: string, nextOpen: boolean) => void;
};

export function CommitRow({ commit, showAttempt = false, open, onToggle }: Props) {
  const now = useNow();
  const attemptSeq = attemptSeqForRow(commit);

  return (
    <li className="task-commit-row" data-commit-open={open ? "true" : "false"}>
      <details
        open={open}
        onToggle={(e) => {
          onToggle(commit.sha, (e.currentTarget as HTMLDetailsElement).open);
        }}
      >
        <summary className="task-commit-row-summary">
          <span className="task-commit-row-chevron" aria-hidden="true" />
          <CommitStatusBadge
            status={commit.status}
            gateReason={commit.gate_reason}
          />
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
        </summary>
        {open ? (
          <div className="task-commit-diff-panel">
            <CommitDiffInline sha={commit.sha} enabled={open} />
          </div>
        ) : null}
      </details>
    </li>
  );
}
