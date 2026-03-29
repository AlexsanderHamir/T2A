import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTask, listTaskEvents } from "@/api";
import type { TaskEventType } from "@/types";
import { userAttention } from "../taskAttention";
import { eventTypeLabel } from "../taskEventLabels";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";
import { taskQueryKeys } from "../queryKeys";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDetailPage({ app }: Props) {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const navigatedAfterDelete = useRef(false);

  useEffect(() => {
    navigatedAfterDelete.current = false;
  }, [taskId]);

  const taskQuery = useQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: ({ signal }) => getTask(taskId, { signal }),
    enabled: Boolean(taskId),
  });

  const eventsQuery = useQuery({
    queryKey: taskQueryKeys.events(taskId),
    queryFn: ({ signal }) => listTaskEvents(taskId, { signal }),
    enabled: Boolean(taskId) && taskQuery.isSuccess,
  });

  useEffect(() => {
    if (!taskId || navigatedAfterDelete.current) return;
    if (
      app.deleteMutation.isSuccess &&
      app.deleteMutation.variables === taskId
    ) {
      navigatedAfterDelete.current = true;
      navigate("/", { replace: true });
    }
  }, [
    taskId,
    app.deleteMutation.isSuccess,
    app.deleteMutation.variables,
    navigate,
  ]);

  if (!taskId) {
    return (
      <p className="muted" role="status">
        Missing task id.
      </p>
    );
  }

  if (taskQuery.isPending) {
    return (
      <p className="muted task-list-phase-msg" role="status">
        Loading task…
      </p>
    );
  }

  if (taskQuery.isError) {
    return (
      <section className="panel task-detail-panel">
        <p className="err-inline" role="alert">
          {taskQuery.error instanceof Error
            ? taskQuery.error.message
            : "Could not load task."}
        </p>
        <p>
          <Link to="/">← Back to tasks</Link>
        </p>
      </section>
    );
  }

  const task = taskQuery.data;
  const events = eventsQuery.data?.events ?? [];
  const attention = userAttention(task, events);

  return (
    <section className="panel task-detail-panel">
      <nav className="task-detail-nav" aria-label="Task navigation">
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
      </nav>

      <header className="task-detail-header">
        <h2 className="task-detail-title">{task.title}</h2>
        <div className="task-detail-meta">
          <span className={statusPillClass(task.status)}>{task.status}</span>
          <span className={priorityPillClass(task.priority)}>
            {task.priority}
          </span>
        </div>
      </header>

      {attention.show ? (
        <div
          className="task-detail-attention"
          role="status"
          aria-live="polite"
        >
          <strong>{attention.headline}</strong>
          <p>{attention.body}</p>
        </div>
      ) : (
        <div className="task-detail-ok" role="status">
          <strong>No action required from you right now.</strong>
          <p className="muted">
            Follow the timeline below for updates. You will see a highlighted
            notice when input or approval is needed.
          </p>
        </div>
      )}

      <div className="task-detail-actions">
        <button
          type="button"
          className="secondary"
          onClick={() => app.openEdit(task)}
          disabled={app.saving}
        >
          Edit task
        </button>
        <button
          type="button"
          className="danger"
          onClick={() => app.requestDelete(task)}
          disabled={app.saving}
        >
          Delete
        </button>
      </div>

      <div className="task-detail-prompt">
        <h3 className="task-detail-subheading">Initial prompt</h3>
        <div
          className="task-detail-prompt-body"
          dangerouslySetInnerHTML={{ __html: task.initial_prompt || "—" }}
        />
      </div>

      <div className="task-detail-timeline">
        <h3 className="task-detail-subheading">Updates</h3>
        {eventsQuery.isPending ? (
          <p className="muted">Loading history…</p>
        ) : eventsQuery.isError ? (
          <p className="err-inline" role="alert">
            {eventsQuery.error instanceof Error
              ? eventsQuery.error.message
              : "Could not load updates."}
          </p>
        ) : events.length === 0 ? (
          <p className="muted">No audit events yet.</p>
        ) : (
          <ol className="task-timeline">
            {events.map((ev) => (
              <li key={ev.seq} className="task-timeline-item">
                <div className="task-timeline-head">
                  <time dateTime={ev.at}>
                    {new Date(ev.at).toLocaleString()}
                  </time>
                  <span className="task-timeline-type">
                    {eventTypeLabel(ev.type)}
                  </span>
                  <span className="task-timeline-by">{ev.by}</span>
                </div>
                <EventDataPreview data={ev.data} eventType={ev.type} />
              </li>
            ))}
          </ol>
        )}
      </div>
    </section>
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
