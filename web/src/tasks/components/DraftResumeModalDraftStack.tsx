import type { TaskDraftSummary } from "@/types";
import { TASK_DRAFTS } from "@/constants/tasks";

const DRAFTS_PER_PAGE = TASK_DRAFTS.resumeModalPerPage;

type DraftListState = "loading" | "empty" | "ready";

type Props = {
  draftListState: DraftListState;
  showLoadingState: boolean;
  contentState: "empty" | "ready";
  visibleDrafts: TaskDraftSummary[];
  draftsTotalCount: number;
  draftPage: number;
  totalDraftPages: number;
  resumePending: boolean;
  onResume: (draftId: string) => void;
  onPreviousPage: () => void;
  onNextPage: () => void;
};

export function DraftResumeModalDraftStack({
  draftListState,
  showLoadingState,
  contentState,
  visibleDrafts,
  draftsTotalCount,
  draftPage,
  totalDraftPages,
  resumePending,
  onResume,
  onPreviousPage,
  onNextPage,
}: Props) {
  return (
    <div
      className={`stack draft-resume-state draft-resume-state--${draftListState}`}
      aria-live="polite"
    >
      <div className="draft-resume-crossfade">
        <div
          className={[
            "draft-resume-layer",
            "draft-resume-layer--loading",
            showLoadingState ? "draft-resume-layer--visible" : "draft-resume-layer--hidden",
          ].join(" ")}
          aria-hidden={!showLoadingState}
        >
          <div className="draft-resume-skeleton" aria-hidden="true">
            {Array.from({ length: DRAFTS_PER_PAGE }).map((_, idx) => (
              <span
                key={`draft-skeleton-${idx}`}
                className="skeleton-block draft-resume-skeleton-row"
              />
            ))}
            <span className="skeleton-block draft-resume-skeleton-meta" />
            <div className="draft-resume-skeleton-actions">
              <span className="skeleton-block draft-resume-skeleton-btn" />
              <span className="skeleton-block draft-resume-skeleton-btn" />
            </div>
          </div>
          <p className="visually-hidden" role="status" aria-live="polite">
            Loading drafts…
          </p>
        </div>
        <div
          className={[
            "draft-resume-layer",
            "draft-resume-layer--content",
            showLoadingState ? "draft-resume-layer--hidden" : "draft-resume-layer--visible",
          ].join(" ")}
          aria-hidden={showLoadingState}
        >
          {contentState === "empty" ? (
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
                  Showing {visibleDrafts.length} of {draftsTotalCount} drafts
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
                      onClick={onPreviousPage}
                    >
                      Previous
                    </button>
                    <button
                      type="button"
                      className="secondary draft-resume-action draft-resume-pager-btn"
                      disabled={resumePending || draftPage + 1 >= totalDraftPages}
                      onClick={onNextPage}
                    >
                      Next
                    </button>
                  </div>
                </div>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
}
