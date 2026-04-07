import { useEffect, useMemo, useRef, useState } from "react";
import { Modal } from "@/shared/Modal";
import type { TaskDraftSummary } from "@/types";

const DRAFTS_PER_PAGE = 5;
const MIN_LOADING_MS = 300;

type Props = {
  drafts: TaskDraftSummary[];
  onStartFresh: () => void;
  onResume: (draftId: string) => void;
  onClose: () => void;
  loading?: boolean;
  loadError?: string | null;
  onRetryLoad?: () => void;
  resumePending?: boolean;
  resumeError?: string | null;
};

export function DraftResumeModal({
  drafts,
  onStartFresh,
  onResume,
  onClose,
  loading = false,
  loadError = null,
  onRetryLoad,
  resumePending = false,
  resumeError = null,
}: Props) {
  const [draftPage, setDraftPage] = useState(0);
  const [showLoadingState, setShowLoadingState] = useState(loading);
  const loadingStartedAtRef = useRef<number | null>(loading ? Date.now() : null);
  const draftListState = showLoadingState
    ? "loading"
    : drafts.length === 0
      ? "empty"
      : "ready";
  const totalDraftPages = Math.max(1, Math.ceil(drafts.length / DRAFTS_PER_PAGE));
  const visibleDrafts = useMemo(() => {
    const start = draftPage * DRAFTS_PER_PAGE;
    return drafts.slice(start, start + DRAFTS_PER_PAGE);
  }, [draftPage, drafts]);

  useEffect(() => {
    if (draftPage < totalDraftPages) return;
    setDraftPage(Math.max(0, totalDraftPages - 1));
  }, [draftPage, totalDraftPages]);

  useEffect(() => {
    if (loading) {
      loadingStartedAtRef.current = Date.now();
      setShowLoadingState(true);
      return;
    }
    if (!showLoadingState) return;
    const startedAt = loadingStartedAtRef.current;
    if (startedAt === null) {
      setShowLoadingState(false);
      return;
    }
    const elapsed = Date.now() - startedAt;
    const remaining = Math.max(0, MIN_LOADING_MS - elapsed);
    if (remaining === 0) {
      setShowLoadingState(false);
      return;
    }
    const timer = window.setTimeout(() => {
      setShowLoadingState(false);
    }, remaining);
    return () => window.clearTimeout(timer);
  }, [loading, showLoadingState]);

  return (
    <Modal onClose={onClose} labelledBy="draft-resume-modal-title" size="wide">
      <section className="panel modal-sheet modal-sheet--edit modal-sheet--draft-resume">
        <h2 id="draft-resume-modal-title">Resume a draft or start fresh</h2>
        <p className="muted">Pick an existing draft to continue, or start a new one.</p>
        {loadError ? (
          <div className="row stack-row-actions">
            <p role="alert">{loadError}</p>
            {onRetryLoad ? (
              <button
                type="button"
                className="secondary"
                onClick={onRetryLoad}
                disabled={loading}
              >
                Retry loading drafts
              </button>
            ) : null}
          </div>
        ) : null}
        {resumeError ? <p role="alert">{resumeError}</p> : null}
        <div
          key={draftListState}
          className={`stack draft-resume-state draft-resume-state--${draftListState}`}
          aria-live="polite"
        >
          {draftListState === "loading" ? (
            <div className="draft-resume-skeleton" aria-hidden="true">
              {Array.from({ length: DRAFTS_PER_PAGE }).map((_, idx) => (
                <span key={`draft-skeleton-${idx}`} className="skeleton-block draft-resume-skeleton-row" />
              ))}
              <span className="skeleton-block draft-resume-skeleton-meta" />
              <div className="draft-resume-skeleton-actions">
                <span className="skeleton-block draft-resume-skeleton-btn" />
                <span className="skeleton-block draft-resume-skeleton-btn" />
              </div>
            </div>
          ) : draftListState === "empty" ? (
            <p className="muted" role="status" aria-live="polite">
              No saved drafts yet. Start fresh to create your first one.
            </p>
          ) : (
            <>
              <div className="draft-resume-list" role="list" aria-label="Saved drafts">
                {visibleDrafts.map((d) => (
                  <button
                    key={d.id}
                    type="button"
                    className="secondary draft-resume-action draft-resume-item"
                    onClick={() => onResume(d.id)}
                    disabled={resumePending}
                  >
                    Resume: {d.name}
                  </button>
                ))}
              </div>
              <div className="draft-resume-footer">
                <p className="muted draft-resume-page-indicator">
                  Showing {visibleDrafts.length} of {drafts.length} drafts
                </p>
                <div className="row stack-row-actions draft-resume-pagination">
                  <p className="muted">
                    Page {draftPage + 1} of {totalDraftPages}
                  </p>
                  <div className="row stack-row-actions draft-resume-pager-actions">
                    <button
                      type="button"
                      className="secondary draft-resume-action draft-resume-pager-btn"
                      disabled={resumePending || draftPage === 0}
                      onClick={() => {
                        setDraftPage((prev) => Math.max(0, prev - 1));
                      }}
                    >
                      Previous
                    </button>
                    <button
                      type="button"
                      className="secondary draft-resume-action draft-resume-pager-btn"
                      disabled={resumePending || draftPage + 1 >= totalDraftPages}
                      onClick={() => {
                        setDraftPage((prev) => Math.min(totalDraftPages - 1, prev + 1));
                      }}
                    >
                      Next
                    </button>
                  </div>
                </div>
              </div>
            </>
          )}
          {draftListState === "loading" ? (
            <p className="visually-hidden" role="status" aria-live="polite">
              Loading drafts…
            </p>
          ) : null}
        </div>
        <div className="row stack-row-actions task-create-modal-actions">
          <button type="button" className="secondary" onClick={onClose} disabled={resumePending}>
            Cancel
          </button>
          <button
            type="button"
            className="task-create-submit"
            onClick={onStartFresh}
            disabled={resumePending}
          >
            Start fresh
          </button>
        </div>
      </section>
    </Modal>
  );
}
