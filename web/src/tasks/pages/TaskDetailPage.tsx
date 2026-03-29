import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTask, listTaskEvents } from "@/api";
import { promptHasVisibleContent } from "../promptFormat";
import { userAttention } from "../taskAttention";
import { TaskUpdatesTimeline } from "../components/TaskUpdatesTimeline";
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
  const timelineEvents = [...events].sort((a, b) => b.seq - a.seq);

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
        <h3 className="task-detail-subheading" id="task-detail-prompt-heading">
          Initial prompt
        </h3>
        {!promptHasVisibleContent(task.initial_prompt) ? (
          <p
            className="muted task-detail-prompt-empty"
            aria-labelledby="task-detail-prompt-heading"
          >
            —
          </p>
        ) : (
          <details className="task-detail-prompt-details">
            <summary className="task-detail-prompt-summary">
              <span className="task-detail-prompt-summary-open-label">
                Show full initial prompt
              </span>
              <span className="task-detail-prompt-summary-close-label">
                Hide initial prompt
              </span>
              <span
                className="task-detail-prompt-summary-chevron"
                aria-hidden="true"
              >
                ▾
              </span>
            </summary>
            <div
              className="task-detail-prompt-body"
              dangerouslySetInnerHTML={{ __html: task.initial_prompt }}
            />
          </details>
        )}
      </div>

      <TaskUpdatesTimeline
        isPending={eventsQuery.isPending}
        isError={eventsQuery.isError}
        error={eventsQuery.error}
        timelineEvents={timelineEvents}
        isEmpty={events.length === 0}
      />
    </section>
  );
}
