import {
  TaskChecklistSkeletonRows,
  TaskTimelineSkeletonItems,
} from "./taskSkeletonChunks";

const TASK_GRAPH_SKELETON_CARDS = 4;
const TASK_DRAFTS_LIST_SKELETON_ROWS = 4;

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

/** Task graph route while the graph query is pending (DS §11). */
export function TaskGraphPageSkeleton() {
  return (
    <section
      className="panel task-graph-page task-graph-skeleton-root"
      aria-busy="true"
    >
      <div className="task-graph-skeleton-nav" role="status" aria-label="Loading task graph">
        <span className="skeleton-block skeleton-block--detail-back" aria-hidden="true" />
      </div>
      <div className="task-graph-skeleton-header" aria-hidden="true">
        <span className="skeleton-block skeleton-block--detail-title" />
        <span className="skeleton-block skeleton-block--detail-line-short" />
      </div>
      <div className="task-graph-skeleton-viewport">
        <div className="task-graph-skeleton-cards">
          {Array.from({ length: TASK_GRAPH_SKELETON_CARDS }, (_, i) => (
            <div key={i} className="task-graph-skeleton-card">
              <span className="skeleton-block skeleton-block--title" />
              <span className="skeleton-block skeleton-block--prompt" />
              <div className="task-graph-skeleton-card-meta">
                <span className="skeleton-block skeleton-block--pill skeleton-block--pill-narrow" />
                <span className="skeleton-block skeleton-block--pill" />
              </div>
            </div>
          ))}
        </div>
      </div>
    </section>
  );
}
