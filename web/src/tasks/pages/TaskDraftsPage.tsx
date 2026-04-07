import { useState } from "react";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useNavigate } from "react-router-dom";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDraftsPage({ app }: Props) {
  useDocumentTitle("Task drafts");
  const navigate = useNavigate();
  const [deletingDraftId, setDeletingDraftId] = useState<string | null>(null);
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
    try {
      await app.deleteDraftByID(draftId);
    } catch {
      // Error state is exposed by the hook and rendered inline on this page.
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

  return (
    <section className="panel">
      <h2>Task drafts</h2>
      <p className="muted">Continue previous drafts or remove ones you no longer need.</p>
      {resumeError ? <p role="alert">{resumeError}</p> : null}
      {deleteError ? <p role="alert">{deleteError}</p> : null}
      <div className="stack">
        {loading ? (
          <p className="muted" role="status" aria-live="polite">
            Loading drafts…
          </p>
        ) : error ? (
          <p role="alert">{error}</p>
        ) : drafts.length === 0 ? (
          <p className="muted">No saved drafts.</p>
        ) : (
          drafts.map((d) => (
            <div key={d.id} className="row stack-row-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => void openDraftInCreateForm(d.id)}
                aria-label={`Open draft ${d.name} in create form`}
                disabled={resumePending || deletePending}
              >
                {resumePending ? "Opening draft…" : `Resume: ${d.name}`}
              </button>
              <button
                type="button"
                className="secondary"
                onClick={() => void deleteDraft(d.id)}
                disabled={resumePending || deletePending}
              >
                {deletePending && deletingDraftId === d.id ? "Deleting…" : "Delete"}
              </button>
            </div>
          ))
        )}
      </div>
    </section>
  );
}
