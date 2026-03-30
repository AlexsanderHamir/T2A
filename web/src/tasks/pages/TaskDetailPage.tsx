import { useQuery } from "@tanstack/react-query";
import { useEffect, useRef, useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { getTask, listTaskEvents } from "@/api";
import { TaskPager } from "../components/TaskPager";
import { promptHasVisibleContent } from "../promptFormat";
import { TASK_EVENTS_PAGE_SIZE } from "../paging";
import { userAttention } from "../taskAttention";
import { statusNeedsUserInput } from "../taskStatusNeedsUser";
import { TaskUpdatesTimeline } from "../components/TaskUpdatesTimeline";
import { priorityPillClass, statusPillClass } from "../taskPillClasses";
import { taskQueryKeys, type TaskEventsCursorKey } from "../queryKeys";
import { useTasksApp } from "../hooks/useTasksApp";

type Props = {
  app: ReturnType<typeof useTasksApp>;
};

export function TaskDetailPage({ app }: Props) {
  const { taskId = "" } = useParams<{ taskId: string }>();
  const navigate = useNavigate();
  const navigatedAfterDelete = useRef(false);

  const [eventsCursor, setEventsCursor] = useState<TaskEventsCursorKey>({
    k: "head",
  });

  useEffect(() => {
    navigatedAfterDelete.current = false;
  }, [taskId]);

  useEffect(() => {
    setEventsCursor({ k: "head" });
  }, [taskId]);

  const taskQuery = useQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: ({ signal }) => getTask(taskId, { signal }),
    enabled: Boolean(taskId),
  });

  const eventsQuery = useQuery({
    queryKey: taskQueryKeys.events(taskId, eventsCursor),
    queryFn: ({ signal }) => {
      const opts: {
        signal?: AbortSignal;
        limit: number;
        beforeSeq?: number;
        afterSeq?: number;
      } = { signal, limit: TASK_EVENTS_PAGE_SIZE };
      if (eventsCursor.k === "before") opts.beforeSeq = eventsCursor.seq;
      if (eventsCursor.k === "after") opts.afterSeq = eventsCursor.seq;
      return listTaskEvents(taskId, opts);
    },
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
  const eventsTotal = eventsQuery.data?.total ?? 0;
  const attention = userAttention(task, {
    approvalPending: eventsQuery.data?.approval_pending ?? false,
  });
  /** API returns newest first when paged. */
  const timelineEvents = events;

  return (
    <section className="panel task-detail-panel">
      <nav className="task-detail-nav" aria-label="Task navigation">
        <Link to="/" className="task-detail-back">
          ← All tasks
        </Link>
      </nav>

      <header className="task-detail-header">
        <h2 className="task-detail-title">{task.title}</h2>
        <p
          className="task-event-detail-stance"
          role="status"
          data-stance={
            statusNeedsUserInput(task.status) ? "needs-user" : "informational"
          }
        >
          {statusNeedsUserInput(task.status)
            ? "Agent needs input"
            : "Informational"}
        </p>
        <div className="task-detail-meta">
          <span
            className={statusPillClass(task.status)}
            data-needs-user={
              statusNeedsUserInput(task.status) ? "true" : undefined
            }
          >
            {task.status}
          </span>
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
          <strong>No agent is waiting on you for this task right now.</strong>
          <p className="muted">
            Follow the timeline for updates. We highlight when an agent needs
            input or approval.
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
        isEmpty={
          !eventsQuery.isPending &&
          !eventsQuery.isError &&
          events.length === 0 &&
          eventsTotal === 0
        }
        taskIdForLinks={taskId}
      />

      {!eventsQuery.isPending &&
      !eventsQuery.isError &&
      eventsTotal > 0 &&
      (eventsQuery.data?.has_more_newer ||
        eventsQuery.data?.has_more_older) ? (
        <TaskPager
          navLabel="Update history pages"
          summary={
            eventsQuery.data?.range_start !== undefined &&
            eventsQuery.data?.range_end !== undefined
              ? `${eventsQuery.data.range_start}–${eventsQuery.data.range_end} of ${eventsTotal}`
              : events.length === 0
                ? "No rows on this page"
                : "—"
          }
          onPrev={() => {
            if (events.length === 0) return;
            const maxSeq = Math.max(...events.map((e) => e.seq));
            setEventsCursor({ k: "after", seq: maxSeq });
          }}
          onNext={() => {
            if (events.length === 0) return;
            const minSeq = Math.min(...events.map((e) => e.seq));
            setEventsCursor({ k: "before", seq: minSeq });
          }}
          disablePrev={eventsQuery.data?.has_more_newer !== true}
          disableNext={eventsQuery.data?.has_more_older !== true}
        />
      ) : null}
    </section>
  );
}
