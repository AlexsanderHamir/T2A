import type { TaskDraftSummary } from "@/types";
import { TASK_DRAFTS } from "@/constants/tasks";
import { formatRelativeTime } from "@/shared/time/relativeTime";

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
  const now = new Date();

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
            <div className="draft-resume-empty" role="status" aria-live="polite">
              <div className="draft-resume-empty__glyph" aria-hidden="true">
                <svg viewBox="0 0 24 24" fill="none" focusable="false">
                  <path
                    d="M7 7.25h10M7 11.25h6M7 15.25h4"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinecap="round"
                  />
                  <path
                    d="M5.75 3.75h8.9L18.25 7.4v12.85H5.75V3.75Z"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinejoin="round"
                  />
                  <path
                    d="M14.25 3.9v3.85h3.85"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinejoin="round"
                  />
                </svg>
              </div>
              <p className="draft-resume-empty__title">No saved drafts yet.</p>
              <p className="draft-resume-empty__text">
                Start fresh — interrupted work autosaves here.
              </p>
            </div>
          ) : (
            <>
              <div className="draft-resume-list" role="list" aria-label="Saved drafts">
                {visibleDrafts.map((d) => {
                  const lastEdited = d.updated_at || d.created_at;
                  const relative = formatRelativeTime(lastEdited, now);
                  return (
                    <button
                      key={d.id}
                      type="button"
                      className="secondary draft-resume-action draft-resume-item"
                      onClick={() => onResume(d.id)}
                      disabled={resumePending}
                      aria-label={`Resume: ${d.name}`}
                    >
                      <span className="draft-resume-item__meta">
                        <span className="draft-resume-item__name">{d.name}</span>
                        {lastEdited && relative ? (
                          <time
                            className="draft-resume-item__time"
                            dateTime={lastEdited}
                            title={lastEdited}
                          >
                            Edited {relative}
                          </time>
                        ) : (
                          <span className="draft-resume-item__time">
                            Ready to continue
                          </span>
                        )}
                      </span>
                      <span className="draft-resume-item__cta" aria-hidden="true">
                        Resume
                      </span>
                    </button>
                  );
                })}
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
