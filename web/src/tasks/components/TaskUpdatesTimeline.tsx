import type { TaskEvent } from "@/types";
import {
  EmptyState,
  EmptyStateTimelineGlyph,
} from "@/shared/EmptyState";
import { TaskTimelineSkeleton } from "./skeletons/taskLoadingSkeletons";
import { TaskUpdatesTimelineEventList } from "./TaskUpdatesTimelineEventList";

export type TaskUpdatesTimelineProps = {
  isPending: boolean;
  isError: boolean;
  error: unknown;
  /** Newest first by seq (display order). */
  timelineEvents: TaskEvent[];
  /** True when the API returned no events (not loading). */
  isEmpty: boolean;
  /** When set, each row links to `/tasks/{id}/events/{seq}`. */
  taskIdForLinks?: string;
  /** Refetch handler shown on the error callout (e.g. `query.refetch`). */
  onRetry?: () => void;
};

export function TaskUpdatesTimeline({
  isPending,
  isError,
  error,
  timelineEvents,
  isEmpty,
  taskIdForLinks,
  onRetry,
}: TaskUpdatesTimelineProps) {
  return (
    <div className="task-detail-section task-detail-timeline">
      <h3 className="task-detail-section-heading" id="task-detail-updates-heading">
        Updates
      </h3>
      {isPending ? (
        <TaskTimelineSkeleton />
      ) : isError ? (
        <div className="err" role="alert">
          <p>
            {error instanceof Error ? error.message : "Could not load updates."}
          </p>
          {onRetry ? (
            <div className="task-detail-error-actions">
              <button type="button" className="secondary" onClick={onRetry}>
                Try again
              </button>
            </div>
          ) : null}
        </div>
      ) : isEmpty ? (
        <EmptyState
          icon={<EmptyStateTimelineGlyph />}
          title="No updates yet"
          description="When agents and the system record changes, they will appear here in order."
        />
      ) : (
        <TaskUpdatesTimelineEventList
          events={timelineEvents}
          taskIdForLinks={taskIdForLinks}
          ariaLabelledBy="task-detail-updates-heading"
        />
      )}
    </div>
  );
}
