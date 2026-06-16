import { useEffect, useRef, useState } from "react";
import { TASK_TIMINGS } from "@/constants/tasks";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { formatRelativeTime } from "@/shared/time/relativeTime";
import { useNavigate } from "react-router-dom";
import { TaskListDeleteGlyph } from "../components/task-list/table/TaskListRowActionIcons";
import { TaskDraftsListSkeleton } from "../components/skeletons";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

function isDraftRowActionExcluded(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return true;
  return Boolean(target.closest("button"));
}

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
  /* Reference "now" for relative-time rendering, captured once per render
     so every row in a paint shows times computed against the same instant.
     React Query refetches the draft list periodically; on each refetch
     the page re-renders and `now` updates naturally — no manual interval
     required. */
  const renderNow = new Date();

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

  const draftCount = drafts.length;
  const hasDrafts = draftCount > 0;

  return (
    <section className="panel task-list-section-panel task-detail-content--enter">
      <header className="task-list-section-head">
        <div className="task-list-section-head__text">
          <h2 id="task-drafts-heading" className="task-list-section-title">
            Task drafts
          </h2>
        </div>
        {hasDrafts ? (
          <div className="task-list-section-actions">
            <span className="draft-count-pill" aria-live="polite">
              <strong>{draftCount}</strong>{" "}
              {draftCount === 1 ? "saved draft" : "saved drafts"}
            </span>
          </div>
        ) : null}
      </header>

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
            ) : !hasDrafts ? (
              <EmptyState
                title="No saved drafts"
                description="Autosaved while you work."
                className="empty-state--task-list-fresh"
                action={{
                  label: "Create a task",
                  onClick: () => {
                    navigate("/");
                    app.openCreateModal();
                  },
                }}
              />
            ) : (
              <ul className="draft-row-list" aria-label="Saved drafts">
                {drafts.map((d) => {
                  const lastEdited = d.updated_at || d.created_at;
                  const relative = formatRelativeTime(lastEdited, renderNow);
                  const isDeleting = deletingDraftId === d.id;
                  const rowDisabled =
                    resumePending ||
                    deletePending ||
                    exitingDraftIds.includes(d.id);
                  return (
                    <li
                      key={d.id}
                      className={[
                        "draft-row",
                        rowDisabled ? "" : "draft-row--interactive",
                        exitingDraftIds.includes(d.id) ? "draft-row--exit" : "",
                      ]
                        .filter(Boolean)
                        .join(" ")}
                      onClick={(e) => {
                        if (rowDisabled || isDraftRowActionExcluded(e.target)) {
                          return;
                        }
                        void openDraftInCreateForm(d.id);
                      }}
                      onKeyDown={(e) => {
                        if (rowDisabled || isDraftRowActionExcluded(e.target)) {
                          return;
                        }
                        if (e.key === "Enter" || e.key === " ") {
                          e.preventDefault();
                          void openDraftInCreateForm(d.id);
                        }
                      }}
                      tabIndex={rowDisabled ? undefined : 0}
                      aria-label={`Resume draft: ${d.name}`}
                      aria-busy={resumePending || undefined}
                    >
                      <div className="draft-row__meta">
                        <span className="draft-row__name" title={d.name}>
                          {d.name}
                        </span>
                        {lastEdited && relative ? (
                          <time
                            className="draft-row__time"
                            dateTime={lastEdited}
                            title={lastEdited}
                          >
                            Edited {relative}
                          </time>
                        ) : null}
                      </div>
                      <div className="draft-row__actions">
                        <div className="task-list-row-actions">
                          <button
                            type="button"
                            className="task-list-icon-btn task-list-icon-btn--delete"
                            aria-label={
                              isDeleting
                                ? `Deleting draft "${d.name}"`
                                : `Delete draft "${d.name}"`
                            }
                            onClick={() => void deleteDraft(d.id)}
                            disabled={rowDisabled}
                            aria-busy={isDeleting || undefined}
                          >
                            <TaskListDeleteGlyph />
                          </button>
                        </div>
                      </div>
                    </li>
                  );
                })}
              </ul>
            )}
          </div>
        ) : null}
      </div>
    </section>
  );
}
