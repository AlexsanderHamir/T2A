import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { getTaskEvent } from "@/api";
import { eventTypeLabel } from "../taskEventLabels";
import { eventTypeNeedsUserInput } from "../taskEventNeedsUser";
import { taskQueryKeys } from "../queryKeys";

export function TaskEventDetailPage() {
  const { taskId = "", eventSeq: eventSeqParam = "" } = useParams<{
    taskId: string;
    eventSeq: string;
  }>();
  const eventSeq = Number.parseInt(eventSeqParam, 10);
  const seqValid = Number.isFinite(eventSeq) && eventSeq >= 1;

  const q = useQuery({
    queryKey: taskQueryKeys.eventDetail(taskId, eventSeq),
    queryFn: ({ signal }) => getTaskEvent(taskId, eventSeq, { signal }),
    enabled: Boolean(taskId) && seqValid,
  });

  if (!taskId) {
    return (
      <p className="muted" role="status">
        Missing task id.
      </p>
    );
  }

  if (!seqValid) {
    return (
      <section className="panel task-detail-panel">
        <p className="err-inline" role="alert">
          Invalid event sequence in the URL.
        </p>
        <p>
          <Link to={`/tasks/${encodeURIComponent(taskId)}`}>
            ← Back to task
          </Link>
        </p>
      </section>
    );
  }

  if (q.isPending) {
    return (
      <p className="muted task-list-phase-msg" role="status">
        Loading event…
      </p>
    );
  }

  if (q.isError) {
    return (
      <section className="panel task-detail-panel">
        <p className="err-inline" role="alert">
          {q.error instanceof Error
            ? q.error.message
            : "Could not load event."}
        </p>
        <p>
          <Link to={`/tasks/${encodeURIComponent(taskId)}`}>
            ← Back to task
          </Link>
        </p>
        <p>
          <Link to="/">← All tasks</Link>
        </p>
      </section>
    );
  }

  const ev = q.data;
  const dataJson = JSON.stringify(ev.data, null, 2);

  return (
    <section className="panel task-detail-panel task-event-detail-panel">
      <nav className="task-detail-nav" aria-label="Event navigation">
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
        <Link
          to={`/tasks/${encodeURIComponent(taskId)}`}
          className="task-event-detail-back-task"
        >
          ← Task
        </Link>
      </nav>

      <header className="task-event-detail-header">
        <h2 className="task-detail-title">Event #{ev.seq}</h2>
        <p
          className="task-event-detail-stance"
          role="status"
          data-stance={
            eventTypeNeedsUserInput(ev.type) ? "needs-user" : "informational"
          }
        >
          {eventTypeNeedsUserInput(ev.type)
            ? "Needs your input"
            : "Informational"}
        </p>
        <p className="muted task-event-detail-task-id">
          Task <code>{ev.task_id}</code>
        </p>
      </header>

      <dl className="task-event-detail-dl">
        <div>
          <dt>Type</dt>
          <dd>
            <code
              className="task-timeline-type-pill"
              data-event-type={ev.type}
              title={eventTypeLabel(ev.type)}
            >
              {ev.type}
            </code>
          </dd>
        </div>
        <div>
          <dt>When</dt>
          <dd>
            <time dateTime={ev.at}>{new Date(ev.at).toLocaleString()}</time>
          </dd>
        </div>
        <div>
          <dt>By</dt>
          <dd className="task-timeline-by">{ev.by}</dd>
        </div>
      </dl>

      <div className="task-event-detail-data-block">
        <h3 className="task-detail-subheading">Data (JSON)</h3>
        <pre className="task-timeline-data task-event-detail-data-pre">
          {dataJson}
        </pre>
      </div>
    </section>
  );
}
