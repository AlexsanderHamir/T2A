import {
  TaskChecklistSkeletonRows,
  TaskTimelineSkeletonItems,
} from "./taskSkeletonChunks";

export function TaskChecklistSkeleton() {
  return (
    <div
      role="status"
      aria-label="Loading checklist"
      className="task-checklist-skeleton-wrap task-checklist-surface-pad"
    >
      <div className="task-checklist-skeleton" aria-hidden="true">
        <TaskChecklistSkeletonRows />
      </div>
    </div>
  );
}

export function TaskTimelineSkeleton() {
  return (
    <div role="status" aria-label="Loading updates" className="task-timeline-skeleton-root">
      <ol className="task-timeline-skeleton" aria-hidden="true">
        <TaskTimelineSkeletonItems />
      </ol>
    </div>
  );
}

/** Event detail route while the event query is pending (DS §11). */
export function TaskEventDetailSkeleton() {
  return (
    <section
      className="panel task-detail-panel task-event-detail-skeleton"
      aria-busy="true"
    >
      <div
        className="task-event-detail-skeleton__inner"
        role="status"
        aria-label="Loading event"
      >
        <div className="task-detail-skeleton-nav">
          <span className="skeleton-block skeleton-block--detail-back" />
        </div>
        <header className="task-event-detail-skeleton-header">
          <span className="skeleton-block skeleton-block--detail-event-title" />
          <div className="task-event-detail-skeleton-meta">
            <span className="skeleton-block skeleton-block--detail-time" />
            <span className="skeleton-block skeleton-block--detail-type-pill" />
            <span className="skeleton-block skeleton-block--detail-by" />
          </div>
        </header>
        <div className="task-event-detail-skeleton-json" aria-hidden="true">
          <span className="skeleton-block skeleton-block--detail-json-line" />
          <span className="skeleton-block skeleton-block--detail-json-line" />
          <span className="skeleton-block skeleton-block--detail-json-line-short" />
        </div>
      </div>
    </section>
  );
}

/** Full-page placeholder while the task detail query is pending (DS §11). */
export function TaskDetailPageSkeleton() {
  return (
    <section
      className="panel task-detail-panel task-detail-page-skeleton"
      aria-busy="true"
    >
      <div
        className="task-detail-page-skeleton__inner"
        role="status"
        aria-label="Loading task"
      >
        <div className="task-detail-skeleton-nav">
          <span className="skeleton-block skeleton-block--detail-back" />
        </div>

        <header className="task-detail-skeleton-header">
          <div className="task-detail-skeleton-header-main">
            <span className="skeleton-block skeleton-block--detail-title" />
            <span className="skeleton-block skeleton-block--detail-stance" />
          </div>
          <div className="task-detail-skeleton-meta">
            <span className="skeleton-block skeleton-block--pill" />
            <span className="skeleton-block skeleton-block--pill skeleton-block--pill-narrow" />
          </div>
        </header>

        <div className="task-detail-skeleton-callout">
          <span className="skeleton-block skeleton-block--detail-line" />
          <span className="skeleton-block skeleton-block--detail-line-short" />
        </div>

        <div className="task-detail-skeleton-actions">
          <span className="skeleton-block skeleton-block--btn" />
          <span className="skeleton-block skeleton-block--btn" />
        </div>

        <div className="task-detail-skeleton-section">
          <div className="task-detail-skeleton-section-head">
            <span className="skeleton-block skeleton-block--detail-heading" />
            <span className="skeleton-block skeleton-block--btn" />
          </div>
          <span className="skeleton-block skeleton-block--detail-line" />
        </div>

        <div className="task-detail-skeleton-section">
          <div className="task-detail-skeleton-section-head">
            <span className="skeleton-block skeleton-block--detail-heading" />
            <span className="skeleton-block skeleton-block--btn" />
          </div>
          <div className="task-checklist-skeleton" aria-hidden="true">
            <TaskChecklistSkeletonRows />
          </div>
        </div>

        <div className="task-detail-skeleton-section">
          <span className="skeleton-block skeleton-block--detail-heading" />
          <span className="skeleton-block skeleton-block--detail-prompt" />
        </div>

        <div className="task-detail-skeleton-section task-detail-skeleton-timeline-wrap">
          <span className="skeleton-block skeleton-block--detail-heading" />
          <ol className="task-timeline-skeleton" aria-hidden="true">
            <TaskTimelineSkeletonItems />
          </ol>
        </div>
      </div>
    </section>
  );
}
