import type { TaskEvent, TaskEventType } from "@/types";
import { Link } from "react-router-dom";
import { eventTypeLabel } from "../taskEventLabels";
import { eventTypeNeedsUserInput } from "../taskEventNeedsUser";
import { awaitingUserReply } from "../taskEventThread";

export function TaskUpdatesTimelineEventList({
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
        const needsUser = eventTypeNeedsUserInput(ev.type);
        const headAndData = (
          <>
            <div className="task-timeline-head">
              <time dateTime={ev.at}>
                {new Date(ev.at).toLocaleString()}
              </time>
              <code
                className="task-timeline-type-pill"
                data-event-type={ev.type}
                data-needs-user={needsUser ? "true" : undefined}
                title={eventTypeLabel(ev.type)}
                aria-label={`${eventTypeLabel(ev.type)}, ${ev.type}`}
              >
                {ev.type}
              </code>
              <span className="task-timeline-by">{ev.by}</span>
              {needsUser && awaitingUserReply(ev) ? (
                <span className="task-timeline-needs-user-hint">
                  Needs your input
                </span>
              ) : null}
            </div>
            <EventDataPreview data={ev.data} eventType={ev.type} />
            {ev.response_thread && ev.response_thread.length > 0 ? (
              <div
                className="task-timeline-thread"
                data-event-response="sent"
              >
                <div className="task-timeline-user-response-head">
                  <span className="task-timeline-user-response-label">
                    Conversation
                  </span>
                </div>
                <ul className="task-timeline-thread-list">
                  {ev.response_thread.map((m, i) => (
                    <li
                      key={`${m.at}-${i}`}
                      className={`task-timeline-thread-msg task-timeline-thread-msg--${m.by}`}
                    >
                      <span className="task-timeline-thread-msg-meta">
                        <strong>
                          {m.by === "agent" ? "Agent" : "You"}
                        </strong>
                        <time dateTime={m.at}>
                          {new Date(m.at).toLocaleString()}
                        </time>
                      </span>
                      <span className="task-timeline-thread-msg-body">
                        {m.body}
                      </span>
                    </li>
                  ))}
                </ul>
              </div>
            ) : ev.user_response ? (
              <div
                className="task-timeline-user-response"
                data-event-response="sent"
              >
                <div className="task-timeline-user-response-head">
                  <span className="task-timeline-user-response-label">
                    Reply to agent
                  </span>
                  {ev.user_response_at ? (
                    <time
                      className="task-timeline-user-response-at"
                      dateTime={ev.user_response_at}
                    >
                      {new Date(ev.user_response_at).toLocaleString()}
                    </time>
                  ) : null}
                </div>
                <span className="task-timeline-user-response-body">
                  {ev.user_response}
                </span>
              </div>
            ) : null}
          </>
        );
        return (
          <li
            key={ev.seq}
            className={
              needsUser
                ? "task-timeline-item task-timeline-item--needs-user"
                : "task-timeline-item"
            }
            data-needs-user={needsUser ? "true" : undefined}
          >
            {taskIdForLinks ? (
              <Link
                className="task-timeline-item-hit"
                to={`/tasks/${encodeURIComponent(taskIdForLinks)}/events/${ev.seq}`}
                aria-label={
                  needsUser
                    ? awaitingUserReply(ev)
                      ? `${eventTypeLabel(ev.type)} — needs your input`
                      : `${eventTypeLabel(ev.type)} — waiting on agent`
                    : undefined
                }
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
  if (eventType === "checklist_inherit_changed") {
    const from = data.from;
    const to = data.to;
    if (typeof from === "boolean" && typeof to === "boolean") {
      return (
        <pre className="task-timeline-data">
          {String(from)} → {String(to)}
        </pre>
      );
    }
  }
  if (eventType === "checklist_item_removed") {
    const text = data.text;
    if (typeof text === "string") {
      return <pre className="task-timeline-data">{text}</pre>;
    }
  }
  if (eventType === "subtask_added" || eventType === "subtask_removed") {
    const title = data.title;
    if (typeof title === "string") {
      return <pre className="task-timeline-data">{title}</pre>;
    }
  }
  return (
    <pre className="task-timeline-data">{JSON.stringify(data, null, 2)}</pre>
  );
}
