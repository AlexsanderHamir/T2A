import type { TaskEvent, TaskEventType } from "@/types";
import { Link } from "react-router-dom";
import { eventTypeLabel } from "../taskEventLabels";

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
};

export function TaskUpdatesTimeline({
  isPending,
  isError,
  error,
  timelineEvents,
  isEmpty,
  taskIdForLinks,
}: TaskUpdatesTimelineProps) {
  return (
    <div className="task-detail-timeline">
      <h3 className="task-detail-subheading" id="task-detail-updates-heading">
        Updates
      </h3>
      {isPending ? (
        <p className="muted">Loading history…</p>
      ) : isError ? (
        <p className="err-inline" role="alert">
          {error instanceof Error ? error.message : "Could not load updates."}
        </p>
      ) : isEmpty ? (
        <p className="muted">No audit events yet.</p>
      ) : (
        <ol
          className="task-timeline"
          aria-labelledby="task-detail-updates-heading"
        >
          {timelineEvents.map((ev) => {
            const headAndData = (
              <>
                <div className="task-timeline-head">
                  <time dateTime={ev.at}>
                    {new Date(ev.at).toLocaleString()}
                  </time>
                  <code
                    className="task-timeline-type-pill"
                    data-event-type={ev.type}
                    title={eventTypeLabel(ev.type)}
                    aria-label={`${eventTypeLabel(ev.type)}, ${ev.type}`}
                  >
                    {ev.type}
                  </code>
                  <span className="task-timeline-by">{ev.by}</span>
                </div>
                <EventDataPreview data={ev.data} eventType={ev.type} />
              </>
            );
            return (
              <li key={ev.seq} className="task-timeline-item">
                {taskIdForLinks ? (
                  <Link
                    className="task-timeline-item-hit"
                    to={`/tasks/${encodeURIComponent(taskIdForLinks)}/events/${ev.seq}`}
                  >
                    {headAndData}
                  </Link>
                ) : (
                  headAndData
                )}
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}

function EventDataPreview({
  data,
  eventType,
}: {
  data: Record<string, unknown>;
  eventType: TaskEventType;
}) {
  const keys = Object.keys(data);
  if (keys.length === 0) return null;
  if (
    eventType === "status_changed" ||
    eventType === "priority_changed" ||
    eventType === "message_added" ||
    eventType === "prompt_appended"
  ) {
    const from = data.from;
    const to = data.to;
    if (typeof from === "string" && typeof to === "string") {
      return (
        <pre className="task-timeline-data">
          {from} → {to}
        </pre>
      );
    }
  }
  return (
    <pre className="task-timeline-data">{JSON.stringify(data, null, 2)}</pre>
  );
}
