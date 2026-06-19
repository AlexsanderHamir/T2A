import { useMemo } from "react";
import { errorMessage } from "@/lib/errorMessage";
import {
  EmptyState,
  EmptyStateTimelineGlyph,
} from "@/shared/EmptyState";
import { useTaskCommits } from "@/tasks/hooks/useTaskCommits";
import { CommitList } from "./CommitList";
import { GitContextMeta } from "./GitContextMeta";

type Props = {
  taskId: string;
  enabled?: boolean;
};

export function TaskCommitsPanel({ taskId, enabled = true }: Props) {
  const commitsQuery = useTaskCommits(taskId, { enabled });
  const commits = useMemo(
    () => commitsQuery.data?.commits ?? [],
    [commitsQuery.data?.commits],
  );

  const gitContext = useMemo(() => {
    if (commits.length === 0) return null;
    const first = commits[0];
    const last = commits[commits.length - 1];
    return {
      repo: first.repo,
      worktree: first.worktree,
      branch: last.branch || first.branch,
    };
  }, [commits]);

  return (
    <section
      className="task-detail-section task-commits-panel"
      data-testid="task-commits-panel"
      aria-labelledby="task-commits-heading"
    >
      <h3 id="task-commits-heading" className="task-detail-section-heading">
        Commits
      </h3>

      {commitsQuery.isPending ? (
        <CommitsLoading />
      ) : commitsQuery.isError ? (
        <div className="err" role="alert">
          <p>
            {errorMessage(
              commitsQuery.error,
              "Could not load commits.",
            )}
          </p>
          <div className="task-detail-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => {
                void commitsQuery.refetch();
              }}
            >
              Try again
            </button>
          </div>
        </div>
      ) : commits.length === 0 ? (
        <EmptyState
          icon={<EmptyStateTimelineGlyph />}
          title="No commits indexed yet"
          description="Commits appear here when an agent run records them in a git worktree."
          density="compact"
          className="task-detail-section-empty empty-state--compact"
        />
      ) : (
        <>
          {gitContext ? <GitContextMeta context={gitContext} /> : null}
          <CommitList taskId={taskId} commits={commits} showAttempt />
        </>
      )}
    </section>
  );
}

function CommitsLoading() {
  return (
    <ul
      className="task-commits-list task-commits-list--loading"
      aria-busy="true"
      aria-label="Loading commits"
    >
      <li className="task-commit-row task-commit-row--skeleton" />
      <li className="task-commit-row task-commit-row--skeleton" />
    </ul>
  );
}

export { CommitStatusBadge } from "./CommitStatusBadge";
