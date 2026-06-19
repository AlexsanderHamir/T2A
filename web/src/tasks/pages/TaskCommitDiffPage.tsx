import { useMemo } from "react";
import { Link, useParams } from "react-router-dom";
import { maxRepoShaQueryBytes } from "@/api/repo";
import { errorMessage } from "@/lib/errorMessage";
import { CopyableId } from "@/shared/CopyableId";
import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useNow } from "@/shared/useNow";
import { CommitDiffPanel } from "../components/task-detail/commits/CommitDiffPanel";
import { CommitStatusBadge } from "../components/task-detail/commits/CommitStatusBadge";
import {
  commitShaParamPattern,
  shortSha,
} from "../components/task-detail/commits/commitDisplay";
import { useCommitDiff } from "../hooks/useCommitDiff";
import { useTaskCommits } from "../hooks/useTaskCommits";

export function TaskCommitDiffPage() {
  const now = useNow();
  const { taskId = "", sha: shaParam = "" } = useParams<{
    taskId: string;
    sha: string;
  }>();
  const sha = decodeURIComponent(shaParam).trim();
  const shaValid =
    sha.length > 0 &&
    sha.length <= maxRepoShaQueryBytes &&
    commitShaParamPattern.test(sha);

  const commitsQuery = useTaskCommits(taskId, { enabled: Boolean(taskId) && shaValid });
  const diffQuery = useCommitDiff(sha, { enabled: Boolean(taskId) && shaValid });
  const commit = useMemo(
    () => commitsQuery.data?.commits.find((c) => c.sha === sha),
    [commitsQuery.data?.commits, sha],
  );

  const pageTitle = shaValid
    ? commit?.message
      ? `${shortSha(sha)}: ${commit.message}`
      : `Commit ${shortSha(sha)}`
    : "Invalid commit";
  useDocumentTitle(pageTitle);

  if (!taskId) {
    return (
      <p className="muted" role="status">
        Missing task id.
      </p>
    );
  }

  if (!shaValid) {
    return (
      <section className="panel task-detail-panel task-detail-content--enter">
        <div className="err" role="alert">
          <p>Invalid commit SHA in the URL.</p>
          <div className="task-detail-error-actions">
            <Link
              to={`/tasks/${encodeURIComponent(taskId)}`}
              className="pd__back project-context-back-link"
            >
              <span aria-hidden="true">&#8249;</span>
              Back to task
            </Link>
          </div>
        </div>
      </section>
    );
  }

  const backTo = `/tasks/${encodeURIComponent(taskId)}`;
  const gitAuthor = diffQuery.data?.author;

  return (
    <section
      className="panel task-detail-panel task-commit-diff-page task-detail-content--enter"
      data-testid="task-commit-diff-page"
    >
      <header className="task-commit-diff-page-head">
        <Link to={backTo} className="pd__back project-context-back-link">
          <span aria-hidden="true">&#8249;</span>
          Back to task
        </Link>

        <div className="task-commit-diff-page-hero">
          {commit?.message ? (
            <h1 className="task-commit-diff-page-message">{commit.message}</h1>
          ) : (
            <h1 className="task-commit-diff-page-message muted">
              {shortSha(sha)}
            </h1>
          )}
          {commit ? (
            <CommitStatusBadge
              status={commit.status}
              gateReason={commit.gate_reason}
            />
          ) : null}
        </div>

        <p className="task-commit-diff-page-meta muted">
          <CopyableId
            value={sha}
            displayValue={shortSha(sha)}
            copyLabel="Copy SHA"
            className="task-commit-diff-page-sha"
          />
          {commit ? (
            <>
              <span className="task-commit-meta-sep" aria-hidden="true">
                ·
              </span>
              <span>{formatRelativeTime(commit.committed_at, new Date(now))}</span>
              {commit.branch ? (
                <>
                  <span className="task-commit-meta-sep" aria-hidden="true">
                    ·
                  </span>
                  <span>{commit.branch}</span>
                </>
              ) : null}
            </>
          ) : commitsQuery.isError ? (
            <>
              <span className="task-commit-meta-sep" aria-hidden="true">
                ·
              </span>
              <span role="status">
                {errorMessage(commitsQuery.error, "Could not load commit metadata.")}
              </span>
            </>
          ) : null}
          {gitAuthor ? (
            <>
              <span className="task-commit-meta-sep" aria-hidden="true">
                ·
              </span>
              <span
                title={
                  diffQuery.data?.author_email
                    ? diffQuery.data.author_email
                    : undefined
                }
              >
                {gitAuthor}
              </span>
            </>
          ) : null}
        </p>
      </header>
      <CommitDiffPanel
        sha={sha}
        viewClassName="task-commit-diff-view task-commit-diff-view--page"
      />
    </section>
  );
}
