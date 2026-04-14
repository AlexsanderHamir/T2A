const CHECKLIST_SKELETON_ROWS = 3;
const TIMELINE_SKELETON_ITEMS = 4;

/** Rows only; parent supplies `div.task-checklist-skeleton` and `aria-hidden` as needed. */
export function TaskChecklistSkeletonRows() {
  return (
    <>
      {Array.from({ length: CHECKLIST_SKELETON_ROWS }, (_, i) => (
        <div key={i} className="task-checklist-skeleton-row">
          <span className="skeleton-block task-checklist-skeleton-check" />
          <span className="skeleton-block task-checklist-skeleton-text" />
        </div>
      ))}
    </>
  );
}

/** List items only; parent supplies `ol.task-timeline-skeleton` and `aria-hidden` as needed. */
export function TaskTimelineSkeletonItems() {
  return (
    <>
      {Array.from({ length: TIMELINE_SKELETON_ITEMS }, (_, i) => (
        <li key={i} className="task-timeline-skeleton-item">
          <div className="task-timeline-skeleton-head">
            <span className="skeleton-block skeleton-block--detail-time" />
            <span className="skeleton-block skeleton-block--detail-type-pill" />
            <span className="skeleton-block skeleton-block--detail-by" />
          </div>
          <span className="skeleton-block skeleton-block--detail-data" />
        </li>
      ))}
    </>
  );
}
