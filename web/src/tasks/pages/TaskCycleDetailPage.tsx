import { useQuery } from "@tanstack/react-query";
import { Link, useParams } from "react-router-dom";
import { listTaskEvents } from "@/api";
import { errorMessage } from "@/lib/errorMessage";
import {
  cycleStatusFillClass,
  cycleStatusLabel,
  formatDurationSeconds,
  formatRunnerModel,
  phaseLabel,
  phaseStatusFillClass,
  phaseStatusLabel,
} from "@/observability";
import { CopyableId } from "@/shared/CopyableId";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import type { TaskCyclePhase, TaskCycleStreamEvent, TaskEvent } from "@/types";
import { eventTypeLabel } from "../task-events";
import { taskQueryKeys } from "../task-query";
import {
  useAgentRunProgress,
  type AgentRunProgressItem,
} from "../hooks/useAgentRunProgress";
import { useTaskCycle, useTaskCycleStream } from "../hooks/useTaskCycles";

export function TaskCycleDetailPage() {
  const { taskId = "", cycleId = "" } = useParams<{
    taskId: string;
    cycleId: string;
  }>();
  const paramsValid = Boolean(taskId) && Boolean(cycleId);
  const cycleQuery = useTaskCycle(taskId, cycleId, { enabled: paramsValid });
  const streamQuery = useTaskCycleStream(taskId, cycleId, {
    enabled: paramsValid,
    limit: 500,
  });
  const auditQuery = useQuery({
    queryKey: taskQueryKeys.events(taskId, { k: "head" }),
    queryFn: ({ signal }) => listTaskEvents(taskId, { signal, limit: 200 }),
    enabled: Boolean(taskId),
  });

  useDocumentTitle(
    cycleQuery.data
      ? `Attempt #${cycleQuery.data.attempt_seq}`
      : paramsValid
        ? "Attempt"
        : "Invalid attempt",
  );

  if (!paramsValid) {
    return (
      <section className="panel task-detail-panel task-detail-content--enter">
        <div className="err" role="alert">
          <p>Missing task or attempt id in the URL.</p>
          <Link to="/">← All tasks</Link>
        </div>
      </section>
    );
  }

  if (cycleQuery.isPending) {
    return (
      <section className="panel task-detail-panel task-attempt-detail task-detail-content--enter">
        <p className="muted" role="status" aria-busy="true">
          Loading attempt…
        </p>
      </section>
    );
  }

  if (cycleQuery.isError) {
    return (
      <section className="panel task-detail-panel task-detail-content--enter">
        <div className="err" role="alert">
          <p>{errorMessage(cycleQuery.error, "Could not load attempt.")}</p>
          <div className="task-detail-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => void cycleQuery.refetch()}
            >
              Try again
            </button>
            <Link to={`/tasks/${encodeURIComponent(taskId)}`}>← Task</Link>
          </div>
        </div>
      </section>
    );
  }

  const cycle = cycleQuery.data;
  const streamEvents = streamQuery.data?.events ?? [];
  const auditEvents =
    auditQuery.data?.events.filter((ev) => ev.data.cycle_id === cycleId) ?? [];

  return (
    <section className="panel task-detail-panel task-attempt-detail task-detail-content--enter">
      <nav className="task-detail-nav" aria-label="Attempt navigation">
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

      <header className="task-attempt-header">
        <div>
          <p className="task-cycle-ticker-eyebrow">Execution attempt</p>
          <h2 className="task-detail-title term-arrow">
            <span>Attempt #{cycle.attempt_seq}</span>
          </h2>
        </div>
        <span className={`cell-pill ${cycleStatusFillClass(cycle.status)}`}>
          {cycleStatusLabel(cycle.status)}
        </span>
      </header>

      <dl className="task-event-detail-dl task-attempt-meta">
        <div>
          <dt>Task</dt>
          <dd>
            <CopyableId value={cycle.task_id} />
          </dd>
        </div>
        <div>
          <dt>Runner</dt>
          <dd>{formatRunnerModel(cycle.cycle_meta)}</dd>
        </div>
        <div>
          <dt>Started</dt>
          <dd>
            <time dateTime={cycle.started_at}>
              {new Date(cycle.started_at).toLocaleString()}
            </time>
          </dd>
        </div>
        <div>
          <dt>Duration</dt>
          <dd>{attemptDurationLabel(cycle.started_at, cycle.ended_at)}</dd>
        </div>
      </dl>

      <section className="task-attempt-section" aria-labelledby="attempt-phases">
        <h3 className="task-detail-subheading term-prompt" id="attempt-phases">
          <span>Phases</span>
        </h3>
        <ol className="task-attempt-phase-list">
          {cycle.phases.map((phase) => (
            <li key={phase.id} className="task-attempt-phase">
              <div className="task-attempt-phase-main">
                <span>{phaseLabel(phase.phase)}</span>
                <span className={`cell-pill ${phaseStatusFillClass(phase.status)}`}>
                  {phaseStatusLabel(phase.status)}
                </span>
              </div>
              {phase.summary ? (
                <p className="muted task-attempt-phase-summary">{phase.summary}</p>
              ) : null}
              <LivePhaseTail
                taskId={taskId}
                cycleId={cycleId}
                phase={phase}
              />
            </li>
          ))}
        </ol>
      </section>

      <section className="task-attempt-section" aria-labelledby="attempt-stream">
        <div className="task-attempt-section-heading-row">
          <h3 className="task-detail-subheading term-prompt" id="attempt-stream">
            <span>Cursor stream</span>
          </h3>
          {streamQuery.isFetching ? <span className="muted">Refreshing…</span> : null}
        </div>
        {streamQuery.isError ? (
          <div className="err" role="alert">
            <p>{errorMessage(streamQuery.error, "Could not load stream events.")}</p>
          </div>
        ) : streamEvents.length === 0 ? (
          <p className="muted">No persisted Cursor updates for this attempt yet.</p>
        ) : (
          <ol className="task-attempt-timeline">
            {streamEvents.map((ev) => (
              <StreamEventRow key={ev.id} ev={ev} />
            ))}
          </ol>
        )}
      </section>

      <section className="task-attempt-section" aria-labelledby="attempt-audit">
        <h3 className="task-detail-subheading term-prompt" id="attempt-audit">
          <span>T2A audit events</span>
        </h3>
        {auditQuery.isPending ? (
          <p className="muted" aria-busy="true">
            Loading audit events…
          </p>
        ) : auditQuery.isError ? (
          <div className="err" role="alert">
            <p>{errorMessage(auditQuery.error, "Could not load audit events.")}</p>
          </div>
        ) : auditEvents.length === 0 ? (
          <p className="muted">No task audit events reference this attempt.</p>
        ) : (
          <ol className="task-attempt-timeline">
            {auditEvents.map((ev) => (
              <AuditEventRow key={ev.seq} taskId={taskId} ev={ev} />
            ))}
          </ol>
        )}
      </section>
    </section>
  );
}

function LivePhaseTail({
  taskId,
  cycleId,
  phase,
}: {
  taskId: string;
  cycleId: string;
  phase: TaskCyclePhase;
}) {
  const live = useAgentRunProgress(taskId, cycleId, phase.phase_seq);
  if (phase.status !== "running" || live.length === 0) return null;
  return (
    <ul className="task-cycle-progress-list" aria-live="polite">
      {live.map((item, i) => (
        <li
          key={`${item.receivedAt}:${i}:${item.progress.kind}`}
          className="task-cycle-progress-item"
        >
          <span className="task-cycle-progress-kind">
            {streamKindLabel(item.progress.kind, item.progress.subtype)}
          </span>
          <span className="task-cycle-progress-message">
            {streamMessage(item)}
          </span>
        </li>
      ))}
    </ul>
  );
}

function StreamEventRow({ ev }: { ev: TaskCycleStreamEvent }) {
  return (
    <li className="task-attempt-timeline-item">
      <article>
        <header className="task-attempt-timeline-header">
          <span className="task-cycle-progress-kind">
            {streamKindLabel(ev.kind, ev.subtype)}
          </span>
          <time dateTime={ev.at}>{new Date(ev.at).toLocaleTimeString()}</time>
        </header>
        <p>{ev.message || ev.tool || "Cursor reported progress."}</p>
        <p className="muted">Phase #{ev.phase_seq}</p>
      </article>
    </li>
  );
}

function AuditEventRow({ taskId, ev }: { taskId: string; ev: TaskEvent }) {
  return (
    <li className="task-attempt-timeline-item">
      <article>
        <header className="task-attempt-timeline-header">
          <Link to={`/tasks/${encodeURIComponent(taskId)}/events/${ev.seq}`}>
            Event #{ev.seq}
          </Link>
          <time dateTime={ev.at}>{new Date(ev.at).toLocaleTimeString()}</time>
        </header>
        <p>{eventTypeLabel(ev.type)}</p>
        <p className="muted">Recorded by {ev.by}</p>
      </article>
    </li>
  );
}

function attemptDurationLabel(startedAt: string, endedAt?: string): string {
  const start = Date.parse(startedAt);
  const end = endedAt ? Date.parse(endedAt) : Date.now();
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) {
    return "Unknown";
  }
  return formatDurationSeconds(Math.round((end - start) / 1000));
}

function streamKindLabel(kind: string, subtype?: string): string {
  if (kind === "tool") return subtype ? `Tool: ${subtype}` : "Tool";
  if (kind === "message") return "Agent note";
  if (kind === "error") return "Error";
  return kind.replace(/_/g, " ");
}

function streamMessage(item: AgentRunProgressItem): string {
  return item.progress.message || item.progress.tool || "Working…";
}
