import { useEffect, useMemo, useRef, useState } from "react";
import { Modal } from "@/shared/Modal";
import { TASK_DRAFTS, TASK_TIMINGS } from "@/constants/tasks";
import type { TaskDraftSummary } from "@/types";
import { DraftResumeModalDraftStack } from "./DraftResumeModalDraftStack";

const DRAFTS_PER_PAGE = TASK_DRAFTS.resumeModalPerPage;
const MIN_LOADING_MS = TASK_TIMINGS.draftResumeMinLoadingMs;

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
  const contentState = drafts.length === 0 ? "empty" : "ready";
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
          onRetryLoad ? (
            <div className="err error-banner" role="alert">
              <span className="error-banner__text">{loadError}</span>
              <button
                type="button"
                className="secondary"
                onClick={onRetryLoad}
                disabled={loading}
              >
                Retry loading drafts
              </button>
            </div>
          ) : (
            <div className="err" role="alert">
              <p>{loadError}</p>
            </div>
          )
        ) : null}
        {resumeError ? (
          <div className="err" role="alert">
            <p>{resumeError}</p>
          </div>
        ) : null}
        <DraftResumeModalDraftStack
          draftListState={draftListState}
          showLoadingState={showLoadingState}
          contentState={contentState}
          visibleDrafts={visibleDrafts}
          draftsTotalCount={drafts.length}
          draftPage={draftPage}
          totalDraftPages={totalDraftPages}
          resumePending={resumePending}
          onResume={onResume}
          onPreviousPage={() => {
            setDraftPage((prev) => Math.max(0, prev - 1));
          }}
          onNextPage={() => {
            setDraftPage((prev) => Math.min(totalDraftPages - 1, prev + 1));
          }}
        />
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
