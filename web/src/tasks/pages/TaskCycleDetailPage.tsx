import { useQuery } from "@tanstack/react-query";
import { useEffect, useId, useState } from "react";
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
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useNow } from "@/shared/useNow";
import type { TaskCyclePhase, TaskCycleStreamEvent, CycleStatus } from "@/types";
import { AttemptAuditTimeline } from "../components/task-detail/attempt/AttemptAuditTimeline";
import { TaskTimelineSkeleton } from "../components/skeletons";
import {
  useAgentRunProgress,
  type AgentRunProgressItem,
} from "../hooks/useAgentRunProgress";
import { useTaskCycle, useTaskCycleStream } from "../hooks/useTaskCycles";
import { taskQueryKeys } from "../task-query";

const STREAM_VISIBLE_INITIAL = 6;
const AUDIT_VISIBLE_INITIAL = 6;
const LOAD_MORE_STEP = 6;

type ActivityTab = "cursor" | "audit";

export function TaskCycleDetailPage() {
  const { taskId = "", cycleId = "" } = useParams<{
    taskId: string;
    cycleId: string;
  }>();
  const paramsValid = Boolean(taskId) && Boolean(cycleId);
  const [activityTab, setActivityTab] = useState<ActivityTab>("cursor");
  const [visibleStreamCount, setVisibleStreamCount] = useState(
    STREAM_VISIBLE_INITIAL,
  );
  const [visibleAuditCount, setVisibleAuditCount] =
    useState(AUDIT_VISIBLE_INITIAL);
  const cursorTabId = useId();
  const auditTabId = useId();
  const cursorPanelId = useId();
  const auditPanelId = useId();

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

  useEffect(() => {
    setVisibleStreamCount(STREAM_VISIBLE_INITIAL);
    setVisibleAuditCount(AUDIT_VISIBLE_INITIAL);
    setActivityTab("cursor");
  }, [cycleId]);

  useDocumentTitle(
    cycleQuery.data
      ? `Attempt #${cycleQuery.data.attempt_seq}`
      : paramsValid
        ? "Attempt"
        : "Invalid attempt",
  );
  const now = useNow({
    enabled: cycleQuery.data?.status === "running" && !cycleQuery.data?.ended_at,
  });

  if (!paramsValid) {
    return (
      <section className="panel task-detail-panel task-detail-content--enter">
        <div className="err" role="alert">
          <p>Missing task or attempt id in the URL.</p>
          <div className="task-detail-error-actions">
            <Link to="/" className="pd__back project-context-back-link">
              <span aria-hidden="true">&#8249;</span>
              All tasks
            </Link>
          </div>
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
            <Link
              to={`/tasks/${encodeURIComponent(taskId)}`}
              className="pd__back project-context-back-link"
            >
              <span aria-hidden="true">&#8249;</span>
              Task
            </Link>
          </div>
        </div>
      </section>
    );
  }

  const cycle = cycleQuery.data;
  // The infinite-query variant exposes `events` flattened across all
  // loaded pages; the spread + sort below produces a stable newest-
  // first view independent of fetch order.
  const streamEvents = [...streamQuery.events].sort(
    (a, b) => b.stream_seq - a.stream_seq,
  );
  const visibleStreamEvents = streamEvents.slice(0, visibleStreamCount);
  const auditEvents = (
    auditQuery.data?.events.filter((ev) => ev.data.cycle_id === cycleId) ?? []
  ).sort((a, b) => b.seq - a.seq);
  const visibleAuditEvents = auditEvents.slice(0, visibleAuditCount);
  const startedParts = formatAttemptStartedParts(cycle.started_at);
  const durationLabel = formatAttemptDurationMeta(
    cycle.started_at,
    cycle.ended_at,
    cycle.status,
    now,
  );
  const showPhaseBadge = cycle.phases.length > 1;
  // Terminal-only endcap: the running case keeps the rail visually open
  // (the running phase's brand-colored marker already conveys liveness),
  // so we only close the rail with a status marker once the attempt has
  // actually reached a terminal state. See AttemptTerminalEndcap below.
  const endcapLabel = attemptEndcapLabel(cycle.status);
  const showEndcap = endcapLabel !== null && cycle.phases.length > 0;
  const endcapTime = showEndcap ? formatAttemptEndedTime(cycle.ended_at) : null;
  // Top bookend: pairs with the terminal endcap so the rail reads
  // "Attempt started → phases → Attempt {completed/failed/aborted}".
  // See AttemptStartCap below.
  const showStartCap = cycle.phases.length > 0;
  const startCapTime = showStartCap
    ? formatAttemptEndedTime(cycle.started_at)
    : null;

  return (
    <section className="panel task-detail-panel task-attempt-detail task-detail-content--enter">
      <nav
        className="task-detail-nav task-attempt-nav"
        aria-label="Attempt navigation"
      >
        <Link
          to="/"
          className="pd__back project-context-back-link task-attempt-nav-link"
        >
          All tasks
        </Link>
        <span className="task-attempt-nav-separator" aria-hidden="true">
          /
        </span>
        <Link
          to={`/tasks/${encodeURIComponent(taskId)}`}
          className="pd__back project-context-back-link task-attempt-nav-link"
        >
          Task
        </Link>
      </nav>

      <header className="task-attempt-header">
        <div className="task-attempt-title-group">
          <div className="task-attempt-title-row">
            <h2 className="task-detail-title">
              Attempt #{cycle.attempt_seq}
            </h2>
            <span className={`cell-pill ${cycleStatusFillClass(cycle.status)}`}>
              {cycleStatusLabel(cycle.status)}
            </span>
          </div>
          <p className="task-attempt-meta-inline">
            <span className="task-attempt-meta-inline-item">
              {formatRunnerModel(cycle.cycle_meta)}
            </span>
            <time
              className="task-attempt-meta-inline-item"
              dateTime={cycle.started_at}
            >
              {startedParts.date} at {startedParts.time}
            </time>
            <span className="task-attempt-meta-inline-item">{durationLabel}</span>
          </p>
        </div>
      </header>

      <section className="task-attempt-section task-attempt-section--phases" aria-labelledby="attempt-phases">
        <h3 className="task-detail-subheading" id="attempt-phases">
          <span>Phases</span>
        </h3>
        <div className="task-attempt-phase-timeline">
          {showStartCap ? (
            <AttemptStartCap
              startedAt={cycle.started_at}
              startedTime={startCapTime}
            />
          ) : null}
          <ol
            className={[
              "task-attempt-phase-track",
              showPhaseBadge && "task-attempt-phase-track--numbered",
              showEndcap && "task-attempt-phase-track--with-endcap",
            ]
              .filter(Boolean)
              .join(" ")}
          >
            {cycle.phases.map((phase, index) => (
              <li
                key={phase.id}
                className="task-attempt-phase-step"
                data-status={phase.status}
                // data-last suppresses the row's outgoing rail connector.
                // When the terminal endcap is rendered below, the last
                // phase row must keep its connector so the rail flows
                // continuously into the endcap marker.
                data-last={
                  !showEndcap && index === cycle.phases.length - 1
                    ? "true"
                    : undefined
                }
              >
                <span className="task-attempt-phase-step-marker" aria-hidden="true" />
                <div className="task-attempt-phase-step-main">
                  <span className="task-attempt-phase-step-name">
                    {phaseLabel(phase.phase)}
                  </span>
                  <span
                    className={`cell-pill ${phaseStatusFillClass(phase.status)}`}
                  >
                    {phaseStatusLabel(phase.status)}
                  </span>
                  {showPhaseBadge ? (
                    <PhaseSeqBadge seq={phase.phase_seq} />
                  ) : null}
                </div>
                <LivePhaseTail taskId={taskId} cycleId={cycleId} phase={phase} />
              </li>
            ))}
          </ol>
          {showEndcap && endcapLabel ? (
            <AttemptTerminalEndcap
              status={cycle.status}
              label={endcapLabel}
              endedAt={cycle.ended_at}
              endedTime={endcapTime}
            />
          ) : null}
        </div>
      </section>

      <section
        className="task-attempt-section task-attempt-section--activity"
        aria-labelledby="attempt-activity-heading"
      >
        <div className="task-attempt-section-heading-row">
          <h3 className="task-detail-subheading" id="attempt-activity-heading">
            <span>Activity</span>
          </h3>
          <div
            className="task-attempt-activity-tabs"
            role="tablist"
            aria-label="Attempt activity views"
          >
            <button
              type="button"
              role="tab"
              id={cursorTabId}
              aria-selected={activityTab === "cursor"}
              aria-controls={cursorPanelId}
              className={
                activityTab === "cursor"
                  ? "task-attempt-activity-tab task-attempt-activity-tab--active"
                  : "task-attempt-activity-tab"
              }
              onClick={() => setActivityTab("cursor")}
            >
              Cursor
              <span className="task-attempt-activity-tab-count">
                {streamEvents.length}
              </span>
            </button>
            <button
              type="button"
              role="tab"
              id={auditTabId}
              aria-selected={activityTab === "audit"}
              aria-controls={auditPanelId}
              className={
                activityTab === "audit"
                  ? "task-attempt-activity-tab task-attempt-activity-tab--active"
                  : "task-attempt-activity-tab"
              }
              onClick={() => setActivityTab("audit")}
            >
              Audit
              <span className="task-attempt-activity-tab-count">
                {auditEvents.length}
              </span>
            </button>
          </div>
        </div>

        {activityTab === "cursor" ? (
          <div
            role="tabpanel"
            id={cursorPanelId}
            aria-labelledby={cursorTabId}
            className="task-attempt-activity-panel"
          >
            {streamQuery.isError ? (
              <div className="err" role="alert">
                <p>
                  {errorMessage(
                    streamQuery.error,
                    "Could not load stream events.",
                  )}
                </p>
              </div>
            ) : streamEvents.length === 0 ? (
              <EmptyState
                title="No Cursor output yet"
                description="Stream lines appear here as the agent runs."
                density="compact"
                hideIcon
              />
            ) : (
              <>
                <ol
                  className={
                    showPhaseBadge
                      ? "task-attempt-stream-list task-attempt-stream-list--numbered"
                      : "task-attempt-stream-list"
                  }
                >
                  {visibleStreamEvents.map((ev) => (
                    <StreamEventRow
                      key={ev.id}
                      ev={ev}
                      showPhaseBadge={showPhaseBadge}
                    />
                  ))}
                </ol>
                <LoadMoreRows
                  shown={visibleStreamEvents.length}
                  total={streamEvents.length}
                  itemLabel="updates"
                  onLoadMore={() =>
                    setVisibleStreamCount((n) => n + LOAD_MORE_STEP)
                  }
                />
              </>
            )}
          </div>
        ) : (
          <div
            role="tabpanel"
            id={auditPanelId}
            aria-labelledby={auditTabId}
            className="task-attempt-activity-panel"
          >
            {auditQuery.isPending ? (
              <TaskTimelineSkeleton />
            ) : auditQuery.isError ? (
              <div className="err" role="alert">
                <p>
                  {errorMessage(auditQuery.error, "Could not load audit events.")}
                </p>
                <div className="task-detail-error-actions">
                  <button
                    type="button"
                    className="secondary"
                    onClick={() => void auditQuery.refetch()}
                  >
                    Try again
                  </button>
                </div>
              </div>
            ) : auditEvents.length === 0 ? (
              <EmptyState
                title="No audit events yet"
                description="System events for this attempt appear here."
                density="compact"
                hideIcon
              />
            ) : (
              <>
                <AttemptAuditTimeline
                  events={visibleAuditEvents}
                  taskId={taskId}
                  ariaLabelledBy={auditTabId}
                />
                <LoadMoreRows
                  shown={visibleAuditEvents.length}
                  total={auditEvents.length}
                  itemLabel="events"
                  onLoadMore={() =>
                    setVisibleAuditCount((n) => n + LOAD_MORE_STEP)
                  }
                />
              </>
            )}
          </div>
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
  const now = useNow({
    enabled: phase.status === "running" && live.length > 0,
    intervalMs: 1000,
  });
  if (phase.status !== "running" || live.length === 0) return null;
  const newestFirst = [...live].sort((a, b) => b.receivedAt - a.receivedAt);
  const latest = newestFirst[0];
  return (
    <div className="task-attempt-live-tail" aria-live="polite">
      <div className="task-attempt-live-tail-heading">
        <span className="task-attempt-live-dot" aria-hidden="true" />
        <span>Live</span>
      </div>
      <ul className="task-cycle-progress-list" aria-label="Recent live updates">
        <li
          className="task-cycle-progress-item task-cycle-progress-item--pending"
          aria-label="Waiting for the next agent update"
        >
          <span className="task-cycle-progress-pulse" aria-hidden="true">
            <span />
            <span />
            <span />
          </span>
          <span className="task-cycle-progress-message">Waiting…</span>
          <span className="task-cycle-progress-time" aria-hidden="true">
            {latest ? `Last ${formatElapsedSince(latest.receivedAt, now)}` : ""}
          </span>
        </li>
        {newestFirst.slice(0, 3).map((item, i) => (
          <li
            key={`${item.receivedAt}:${i}:${item.progress.kind}:${item.progress.subtype ?? ""}`}
            className={`task-cycle-progress-item${i === 0 ? " task-cycle-progress-item--latest" : ""}`}
          >
            <span className="task-cycle-progress-kind">
              {streamKindLabel(
                item.progress.kind,
                item.progress.subtype,
                item.progress.tool,
              )}
            </span>
            <span className="task-cycle-progress-message">
              {streamMessage(item)}
            </span>
            <time
              className="task-cycle-progress-time"
              dateTime={new Date(item.receivedAt).toISOString()}
            >
              {formatLiveProgressTime(item.receivedAt)}
            </time>
          </li>
        ))}
      </ul>
    </div>
  );
}

function LoadMoreRows({
  shown,
  total,
  itemLabel,
  onLoadMore,
}: {
  shown: number;
  total: number;
  itemLabel: string;
  onLoadMore: () => void;
}) {
  if (shown >= total) {
    return (
      <p className="task-attempt-count muted">
        All {total} {itemLabel} shown.
      </p>
    );
  }
  return (
    <div className="task-attempt-load-more">
      <p className="task-attempt-count muted">
        {shown} of {total} {itemLabel}
      </p>
      <button type="button" className="secondary" onClick={onLoadMore}>
        Load more
      </button>
    </div>
  );
}

function PhaseSeqBadge({ seq }: { seq: number }) {
  return (
    <span className="task-attempt-phase-seq" aria-label={`Phase ${seq}`}>
      PHASE {seq}
    </span>
  );
}

/**
 * Closes the phase rail with a single marker representing the cycle's
 * terminal outcome. The phase track on its own ends abruptly at the last
 * phase row, leaving the reader to look up at the header pill to learn
 * how the attempt as a whole ended; this endcap puts the rollup at the
 * natural end of the timeline. Only rendered for terminal statuses —
 * running attempts intentionally leave the rail open so the brand-color
 * halo on the running phase remains the dominant liveness signal.
 */
function AttemptTerminalEndcap({
  status,
  label,
  endedAt,
  endedTime,
}: {
  status: CycleStatus;
  label: string;
  endedAt: string | undefined;
  endedTime: string | null;
}) {
  return (
    <div
      className="task-attempt-phase-endcap"
      data-status={status}
      aria-label={
        endedTime ? `${label} at ${endedTime}` : label
      }
    >
      <span className="task-attempt-phase-endcap-marker" aria-hidden="true" />
      <span className="task-attempt-phase-endcap-name">{label}</span>
      {endedTime && endedAt ? (
        <time className="task-attempt-phase-endcap-time" dateTime={endedAt}>
          {endedTime}
        </time>
      ) : null}
    </div>
  );
}

/**
 * Opens the phase rail so the timeline reads as a complete arc:
 * "Attempt started → phases → Attempt {completed/failed/aborted}".
 * Always rendered when the cycle has phases.
 */
function AttemptStartCap({
  startedAt,
  startedTime,
}: {
  startedAt: string;
  startedTime: string | null;
}) {
  const label = "Attempt started";
  return (
    <div
      className="task-attempt-phase-startcap"
      aria-label={startedTime ? `${label} at ${startedTime}` : label}
    >
      <span className="task-attempt-phase-startcap-marker" aria-hidden="true" />
      <span className="task-attempt-phase-startcap-name">{label}</span>
      {startedTime ? (
        <time className="task-attempt-phase-startcap-time" dateTime={startedAt}>
          {startedTime}
        </time>
      ) : null}
    </div>
  );
}

function attemptEndcapLabel(status: CycleStatus): string | null {
  switch (status) {
    case "succeeded":
      return "Attempt completed";
    case "failed":
      return "Attempt failed";
    case "aborted":
      return "Attempt aborted";
    default:
      return null;
  }
}

function formatAttemptEndedTime(
  endedAt: string | undefined,
): string | null {
  if (!endedAt) return null;
  const d = new Date(endedAt);
  if (!Number.isFinite(d.getTime())) return null;
  return d.toLocaleTimeString(undefined, {
    hour: "numeric",
    minute: "2-digit",
  });
}

function StreamEventRow({
  ev,
  showPhaseBadge,
}: {
  ev: TaskCycleStreamEvent;
  showPhaseBadge: boolean;
}) {
  const preview = ev.message || ev.tool || "Agent reported progress.";
  const kind = streamKindDescriptor(ev.kind, ev.subtype, ev.tool);
  return (
    <li className="task-attempt-stream-row">
      <details className="task-attempt-stream-details">
        <summary className="task-attempt-stream-summary">
          <time className="task-attempt-stream-time" dateTime={ev.at}>
            {new Date(ev.at).toLocaleTimeString(undefined, {
              hour: "numeric",
              minute: "2-digit",
            })}
          </time>
          <span className="task-attempt-stream-label">
            <span
              className={`task-attempt-stream-kind task-attempt-stream-kind--${kind.tone}`}
              title={kind.title}
            >
              {kind.label}
            </span>
            <span className="task-attempt-stream-message" title={preview}>
              {preview}
            </span>
          </span>
          {showPhaseBadge ? <PhaseSeqBadge seq={ev.phase_seq} /> : null}
        </summary>
        <div className="task-attempt-stream-detail-panel">
          <dl className="task-attempt-stream-detail-list">
            {ev.tool ? (
              <div>
                <dt>Tool</dt>
                <dd>{ev.tool}</dd>
              </div>
            ) : null}
            <div>
              <dt>Phase</dt>
              <dd>#{ev.phase_seq}</dd>
            </div>
          </dl>
          <div className="task-attempt-stream-detail-block">
            <h4>Raw payload</h4>
            <pre>{JSON.stringify(ev.payload, null, 2)}</pre>
          </div>
        </div>
      </details>
    </li>
  );
}

function formatAttemptDurationMeta(
  startedAt: string,
  endedAt: string | undefined,
  status: CycleStatus,
  now: number,
): string {
  const start = Date.parse(startedAt);
  const end = endedAt ? Date.parse(endedAt) : now;
  if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) {
    return "Unknown duration";
  }
  const duration = formatDurationSeconds(Math.round((end - start) / 1000));
  const running = status === "running" || !endedAt;
  return running ? `Running for ${duration}` : `Ran for ${duration}`;
}

function formatAttemptStartedParts(startedAt: string): {
  date: string;
  time: string;
} {
  const started = new Date(startedAt);
  return {
    date: started.toLocaleDateString(undefined, {
      month: "short",
      day: "numeric",
      year: "numeric",
    }),
    time: started.toLocaleTimeString(undefined, {
      hour: "numeric",
      minute: "2-digit",
    }),
  };
}

type StreamKindTone =
  | "reply"
  | "tool"
  | "done"
  | "failed"
  | "session"
  | "error"
  | "neutral";

type StreamKindDescriptor = {
  label: string;
  title: string;
  tone: StreamKindTone;
};

function streamKindDescriptor(
  kind: string,
  subtype?: string,
  tool?: string,
): StreamKindDescriptor {
  const toolName = tool?.trim();
  if (kind === "tool_call" || kind === "tool") {
    if (subtype === "completed" || subtype === "success" || subtype === "done") {
      return {
        label: "Tool done",
        title: toolName
          ? `Tool finished successfully: ${toolName}`
          : "Cursor tool finished successfully",
        tone: "done",
      };
    }
    if (subtype === "failed" || subtype === "error") {
      return {
        label: "Tool failed",
        title: toolName
          ? `Tool returned an error: ${toolName}`
          : "Cursor tool returned an error",
        tone: "failed",
      };
    }
    return {
      label: "Tool call",
      title: toolName
        ? `Cursor invoked a tool: ${toolName}`
        : "Cursor started running a tool",
      tone: "tool",
    };
  }
  if (kind === "assistant" || kind === "message") {
    return {
      label: "Agent reply",
      title: "Message from the Cursor agent",
      tone: "reply",
    };
  }
  if (kind === "system") {
    return {
      label: "Session",
      title: "Cursor CLI session event",
      tone: "session",
    };
  }
  if (kind === "error") {
    return {
      label: "Error",
      title: "Cursor stream reported an error",
      tone: "error",
    };
  }
  const normalized = kind.replace(/_/g, " ");
  return {
    label: normalized,
    title: `Cursor stream event: ${normalized}`,
    tone: "neutral",
  };
}

function streamKindLabel(kind: string, subtype?: string, tool?: string): string {
  return streamKindDescriptor(kind, subtype, tool).label;
}

function streamMessage(item: AgentRunProgressItem): string {
  return item.progress.message || item.progress.tool || "Working…";
}

function formatLiveProgressTime(receivedAt: number): string {
  return new Date(receivedAt).toLocaleTimeString(undefined, {
    hour: "numeric",
    minute: "2-digit",
  });
}

function formatElapsedSince(receivedAt: number, now: number): string {
  const elapsedSeconds = Math.max(0, Math.floor((now - receivedAt) / 1000));
  if (elapsedSeconds < 1) return "just now";
  if (elapsedSeconds < 60) return `${elapsedSeconds}s ago`;
  return `${Math.floor(elapsedSeconds / 60)}m ago`;
}
