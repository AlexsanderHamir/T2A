const TASK_DRAFTS_LIST_SKELETON_ROWS = 4;

/** Drafts list route while the drafts query is pending (layout matches `.draft-list-row`). */
export function TaskDraftsListSkeleton() {
  return (
    <div
      className="stack"
      role="status"
      aria-label="Loading drafts"
      aria-busy="true"
    >
      {Array.from({ length: TASK_DRAFTS_LIST_SKELETON_ROWS }, (_, i) => (
        <div key={i} className="row stack-row-actions" aria-hidden="true">
          <span className="skeleton-block skeleton-block--btn" />
          <span className="skeleton-block skeleton-block--btn" />
        </div>
      ))}
    </div>
  );
}
