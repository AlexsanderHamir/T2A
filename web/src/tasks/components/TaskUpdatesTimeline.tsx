import { useMemo } from "react";
import type { TaskEvent, TaskEventType } from "@/types";
import { Link } from "react-router-dom";
import { eventTypeLabel } from "../taskEventLabels";
import { eventTypeNeedsUserInput } from "../taskEventNeedsUser";

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
  const { needsUser, other } = useMemo(() => {
    const needsUser: TaskEvent[] = [];
    const other: TaskEvent[] = [];
    for (const ev of timelineEvents) {
      (eventTypeNeedsUserInput(ev.type) ? needsUser : other).push(ev);
    }
    return { needsUser, other };
  }, [timelineEvents]);

  const splitIntoTwo =
    needsUser.length > 0 && other.length > 0;

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
      ) : splitIntoTwo ? (
        <div
          className="task-timeline-split"
          aria-labelledby="task-detail-updates-heading"
        >
          <section
            className="task-timeline-section task-timeline-section--needs-user"
            aria-labelledby="task-timeline-needs-user-heading"
          >
            <h4
              className="task-timeline-section-title"
              id="task-timeline-needs-user-heading"
            >
              Needs your input
            </h4>
            <TimelineEventList
              events={needsUser}
              taskIdForLinks={taskIdForLinks}
              ariaLabelledBy="task-timeline-needs-user-heading"
            />
          </section>
          <section
            className="task-timeline-section"
            aria-labelledby="task-timeline-other-heading"
          >
            <h4
              className="task-timeline-section-title"
              id="task-timeline-other-heading"
            >
              Other activity
            </h4>
            <TimelineEventList
              events={other}
              taskIdForLinks={taskIdForLinks}
              ariaLabelledBy="task-timeline-other-heading"
            />
          </section>
        </div>
      ) : (
        <TimelineEventList
          events={timelineEvents}
          taskIdForLinks={taskIdForLinks}
          ariaLabelledBy="task-detail-updates-heading"
        />
      )}
    </div>
  );
}

function TimelineEventList({
  events,
  taskIdForLinks,
  ariaLabelledBy,
}: {
  events: TaskEvent[];
  taskIdForLinks?: string;
  ariaLabelledBy?: string;
}) {
  return (
    <ol
      className="task-timeline"
      {...(ariaLabelledBy
        ? { "aria-labelledby": ariaLabelledBy }
        : { "aria-label": "Audit events" })}
    >
      {events.map((ev) => {
        const headAndData = (
          <>
            <div className="task-timeline-head">
              <time dateTime={ev.at}>
                {new Date(ev.at).toLocaleString()}
              </time>
              <code
                className="task-timeline-type-pill"
                data-event-type={ev.type}
                data-needs-user={
                  eventTypeNeedsUserInput(ev.type) ? "true" : undefined
                }
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
