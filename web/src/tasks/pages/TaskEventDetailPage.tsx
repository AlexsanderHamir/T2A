import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useState } from "react";
import { Link, useParams } from "react-router-dom";
import { getTaskEvent, patchTaskEventUserResponse } from "@/api";
import { FieldRequirementBadge } from "@/shared/FieldLabel";
import { eventTypeLabel } from "../taskEventLabels";
import { eventTypeNeedsUserInput } from "../taskEventNeedsUser";
import { awaitingUserReply } from "../taskEventThread";
import { taskQueryKeys } from "../queryKeys";

export function TaskEventDetailPage() {
  const qc = useQueryClient();
  const { taskId = "", eventSeq: eventSeqParam = "" } = useParams<{
    taskId: string;
    eventSeq: string;
  }>();
  const eventSeq = Number.parseInt(eventSeqParam, 10);
  const seqValid = Number.isFinite(eventSeq) && eventSeq >= 1;

  const [draft, setDraft] = useState("");

  const q = useQuery({
    queryKey: taskQueryKeys.eventDetail(taskId, eventSeq),
    queryFn: ({ signal }) => getTaskEvent(taskId, eventSeq, { signal }),
    enabled: Boolean(taskId) && seqValid,
  });

  const saveMutation = useMutation({
    mutationFn: (text: string) =>
      patchTaskEventUserResponse(taskId, eventSeq, text),
    onSuccess: (updated) => {
      qc.setQueryData(taskQueryKeys.eventDetail(taskId, eventSeq), updated);
      void qc.invalidateQueries({
        queryKey: [...taskQueryKeys.all, "detail", taskId],
      });
      setDraft("");
    },
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
  const needsInput = eventTypeNeedsUserInput(ev.type);
  const awaitingUser = needsInput && awaitingUserReply(ev);

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
            needsInput ? "needs-user" : "informational"
          }
          data-awaiting-response={awaitingUser ? "true" : undefined}
        >
          {needsInput
            ? awaitingUser
              ? "Agent needs input"
              : "You replied — waiting on agent"
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
              data-needs-user={needsInput ? "true" : undefined}
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

      {needsInput ? (
        <div className="task-event-detail-response-block">
          <div className="field-heading-with-req task-event-response-heading-row">
            <h3
              className="task-detail-subheading"
              id="task-event-response-heading"
            >
              Add a message
            </h3>
            <FieldRequirementBadge requirement="required" />
          </div>
          <p className="muted task-event-detail-thread-hint">
            Each send appends to this conversation and appears on the task timeline.
          </p>
          {saveMutation.isError ? (
            <p className="err-inline" role="alert">
              {saveMutation.error instanceof Error
                ? saveMutation.error.message
                : "Could not send message."}
            </p>
          ) : null}
          <textarea
            id="task-event-user-response"
            className="task-event-detail-response-field"
            rows={5}
            value={draft}
            onChange={(e) => setDraft(e.target.value)}
            disabled={saveMutation.isPending}
            aria-labelledby="task-event-response-heading"
            aria-required="true"
            placeholder="Type a message and send. It is stored on this event and shown on the task timeline."
          />
          <div className="task-event-detail-response-actions">
            <button
              type="button"
              onClick={() => {
                const t = draft.trim();
                if (t) saveMutation.mutate(t);
              }}
              disabled={saveMutation.isPending || !draft.trim()}
            >
              {saveMutation.isPending ? "Sending…" : "Send"}
            </button>
          </div>
        </div>
      ) : null}

      <div className="task-event-detail-data-block">
        <h3 className="task-detail-subheading">Data (JSON)</h3>
        <pre className="task-timeline-data task-event-detail-data-pre">
          {dataJson}
        </pre>
      </div>

      {ev.response_thread && ev.response_thread.length > 0 ? (
        <div
          className="task-event-detail-thread"
          role="log"
          aria-label="Conversation on this event"
        >
          <h3 className="task-detail-subheading" id="task-event-thread-heading">
            Conversation
          </h3>
          <ul
            className="task-event-detail-thread-list"
            aria-labelledby="task-event-thread-heading"
          >
            {ev.response_thread.map((m, i) => (
              <li
                key={`${m.at}-${i}`}
                className={`task-event-detail-thread-item task-event-detail-thread-item--${m.by}`}
              >
                <article className="task-event-detail-thread-bubble">
                  <header className="task-event-detail-thread-meta">
                    <span className="task-event-detail-thread-by">
                      {m.by === "agent" ? "Agent" : "You"}
                    </span>
                    <span
                      className="task-event-detail-thread-meta-sep"
                      aria-hidden="true"
                    >
                      ·
                    </span>
                    <time
                      className="task-event-detail-thread-time"
                      dateTime={m.at}
                    >
                      {new Date(m.at).toLocaleString()}
                    </time>
                  </header>
                  <p className="task-event-detail-thread-body">{m.body}</p>
                </article>
              </li>
            ))}
          </ul>
        </div>
      ) : null}
    </section>
  );
}
