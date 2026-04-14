import { useEffect, useRef, useState } from "react";
import { TASK_TIMINGS } from "@/constants/tasks";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useNavigate } from "react-router-dom";
import { TaskDraftsListSkeleton } from "../components/taskLoadingSkeletons";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDraftsPage({ app }: Props) {
  useDocumentTitle("Task drafts");
  const navigate = useNavigate();
  const [deletingDraftId, setDeletingDraftId] = useState<string | null>(null);
  const [exitingDraftIds, setExitingDraftIds] = useState<string[]>([]);
  const deleteTimerRef = useRef<number | null>(null);
  const openDraftInCreateForm = async (draftId: string) => {
    try {
      await app.resumeDraftByID(draftId);
      navigate("/");
    } catch {
      // Error state is exposed by the hook and rendered inline on this page.
    }
  };
  const deleteDraft = async (draftId: string) => {
    setDeletingDraftId(draftId);
    setExitingDraftIds((current) =>
      current.includes(draftId) ? current : [...current, draftId],
    );
    await new Promise<void>((resolve) => {
      deleteTimerRef.current = window.setTimeout(() => {
        deleteTimerRef.current = null;
        resolve();
      }, TASK_TIMINGS.draftDeleteExitMs);
    });
    try {
      await app.deleteDraftByID(draftId);
    } catch {
      // Error state is exposed by the hook and rendered inline on this page.
      setExitingDraftIds((current) => current.filter((id) => id !== draftId));
    } finally {
      setDeletingDraftId((current) => (current === draftId ? null : current));
    }
  };
  const loading = app.draftListLoading;
  const error = app.draftListError;
  const drafts = app.taskDrafts;
  const resumePending = app.resumeDraftPending;
  const resumeError = app.resumeDraftError;
  const deletePending = app.deleteDraftPending;
  const deleteError = app.deleteDraftError;
  const showDraftsSkeleton = useDelayedTrue(
    loading,
    TASK_TIMINGS.draftResumeMinLoadingMs,
  );

  useEffect(() => {
    const draftIds = new Set(drafts.map((d) => d.id));
    setExitingDraftIds((current) => current.filter((id) => draftIds.has(id)));
  }, [drafts]);

  useEffect(() => {
    return () => {
      if (deleteTimerRef.current !== null) {
        window.clearTimeout(deleteTimerRef.current);
      }
    };
  }, []);

  return (
    <section className="panel task-detail-content--enter">
      <h2>Task drafts</h2>
      <p className="muted">Continue previous drafts or remove ones you no longer need.</p>
      {resumeError ? (
        <div className="err" role="alert">
          <p>{resumeError}</p>
        </div>
      ) : null}
      {deleteError ? (
        <div className="err" role="alert">
          <p>{deleteError}</p>
        </div>
      ) : null}
      <div className="stack">
        {loading && showDraftsSkeleton ? <TaskDraftsListSkeleton /> : null}
        {!loading ? (
          <div className="stack task-list-content task-list-content--enter">
            {error ? (
              <div className="err" role="alert">
                <p>{error}</p>
                <div className="task-detail-error-actions">
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => {
                      void app.retryDraftList();
                    }}
                  >
                    Try again
                  </button>
                </div>
              </div>
            ) : drafts.length === 0 ? (
              <EmptyState
                title="No saved drafts"
                description="Create a task from home and your work can be saved as a draft automatically."
                action={{
                  label: "Create a task",
                  onClick: () => {
                    navigate("/");
                    app.openCreateModal();
                  },
                }}
              />
            ) : (
              drafts.map((d) => (
                <div
                  key={d.id}
                  className={[
                    "row",
                    "stack-row-actions",
                    "draft-list-row",
                    exitingDraftIds.includes(d.id) ? "draft-list-row--exit" : "",
                  ]
                    .filter(Boolean)
                    .join(" ")}
                >
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void openDraftInCreateForm(d.id)}
                    aria-label={`Open draft ${d.name} in create form`}
                    disabled={
                      resumePending ||
                      deletePending ||
                      exitingDraftIds.includes(d.id)
                    }
                  >
                    {resumePending ? "Opening draft…" : `Resume: ${d.name}`}
                  </button>
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void deleteDraft(d.id)}
                    disabled={
                      resumePending ||
                      deletePending ||
                      exitingDraftIds.includes(d.id)
                    }
                  >
                    {deletingDraftId === d.id ? "Deleting…" : "Delete"}
                  </button>
                </div>
              ))
            )}
          </div>
        ) : null}
      </div>
    </section>
  );
}
