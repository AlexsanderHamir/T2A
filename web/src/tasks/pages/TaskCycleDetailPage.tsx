import { useQuery } from "@tanstack/react-query";
import { useEffect, useId, useState, type Dispatch, type SetStateAction } from "react";
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
import type {
  CycleStatus,
  TaskCycleDetail,
  TaskCyclePhase,
  TaskCycleStreamEvent,
} from "@/types";
import type { UseQueryResult } from "@tanstack/react-query";
import type { UseTaskCycleStreamResult } from "../hooks/useTaskCycles";
import { AttemptAuditTimeline } from "../components/task-detail/attempt/AttemptAuditTimeline";
import { CycleLiveProgressList } from "../components/task-detail/cycles/CycleLiveProgressList";
import { TaskTimelineSkeleton } from "../components/skeletons";
import {
  useAgentRunProgress,
} from "../hooks/useAgentRunProgress";
import { useTaskCycle, useTaskCycleStream } from "../hooks/useTaskCycles";
import { agentProgressKindDescriptor } from "../cycleDisplay/agentProgressDisplay";
import { taskQueryKeys } from "../task-query";
import {
  activityCountCaption,
  filterAuditEventsByPhase,
  filterStreamEventsByPhase,
} from "./attempt/filterActivityByPhase";
import { useAttemptPhaseFilter } from "./attempt/useAttemptPhaseFilter";
import { PhaseDebugDetails } from "./attempt/PhaseDebugDetails";

const STREAM_VISIBLE_INITIAL = 6;
const AUDIT_VISIBLE_INITIAL = 6;
const LOAD_MORE_STEP = 6;

type ActivityTab = "cursor" | "audit";

type TaskCycleDetailPageState = {
  taskId: string;
  cycleId: string;
  paramsValid: boolean;
  activityTab: ActivityTab;
  setActivityTab: (tab: ActivityTab) => void;
  visibleStreamCount: number;
  setVisibleStreamCount: Dispatch<SetStateAction<number>>;
  visibleAuditCount: number;
  setVisibleAuditCount: Dispatch<SetStateAction<number>>;
  cursorTabId: string;
  auditTabId: string;
  cursorPanelId: string;
  auditPanelId: string;
  cycleQuery: UseQueryResult<TaskCycleDetail, Error>;
  streamQuery: UseTaskCycleStreamResult;
  auditQuery: UseQueryResult<
    Awaited<ReturnType<typeof listTaskEvents>>,
    Error
  >;
  now: number;
};

type AttemptTimelineDisplay = {
  startedParts: ReturnType<typeof formatAttemptStartedParts>;
  durationLabel: string;
  showPhaseBadge: boolean;
  endcapLabel: string | null;
  showEndcap: boolean;
  endcapTime: string | null;
  showStartCap: boolean;
  startCapTime: string | null;
};

export function TaskCycleDetailPage() {
  const pageState = useTaskCycleDetailPageState();

  if (!pageState.paramsValid) {
    return <TaskCycleInvalidParamsSection />;
  }
  if (pageState.cycleQuery.isPending) {
    return <TaskCycleLoadingSection />;
  }
  if (pageState.cycleQuery.isError) {
    return (
      <TaskCycleErrorSection
        taskId={pageState.taskId}
        error={pageState.cycleQuery.error}
        onRetry={() => void pageState.cycleQuery.refetch()}
      />
    );
  }

  return <TaskCycleDetailLoadedSection pageState={pageState} cycle={pageState.cycleQuery.data} />;
}

function useTaskCycleDetailPageState(): TaskCycleDetailPageState {
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

  return {
    taskId,
    cycleId,
    paramsValid,
    activityTab,
    setActivityTab,
    visibleStreamCount,
    setVisibleStreamCount,
    visibleAuditCount,
    setVisibleAuditCount,
    cursorTabId,
    auditTabId,
    cursorPanelId,
    auditPanelId,
    cycleQuery,
    streamQuery,
    auditQuery,
    now,
  };
}

function TaskCycleInvalidParamsSection() {
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

function TaskCycleLoadingSection() {
  return (
    <section className="panel task-detail-panel task-attempt-detail task-detail-content--enter">
      <p className="muted" role="status" aria-busy="true">
        Loading attempt…
      </p>
    </section>
  );
}

function TaskCycleErrorSection({
  taskId,
  error,
  onRetry,
}: {
  taskId: string;
  error: Error;
  onRetry: () => void;
}) {
  return (
    <section className="panel task-detail-panel task-detail-content--enter">
      <div className="err" role="alert">
        <p>{errorMessage(error, "Could not load attempt.")}</p>
        <div className="task-detail-error-actions">
          <button type="button" className="secondary" onClick={onRetry}>
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

function TaskCycleDetailLoadedSection({
  pageState,
  cycle,
}: {
  pageState: TaskCycleDetailPageState;
  cycle: TaskCycleDetail;
}) {
  const timelineDisplay = buildAttemptTimelineDisplay(cycle, pageState.now);
  const phaseFilter = useAttemptPhaseFilter(cycle.phases);
  const allStreamEvents = sortStreamEventsNewestFirst(pageState.streamQuery.events);
  const allAuditEvents = filterAuditEventsForCycle(
    pageState.auditQuery.data?.events,
    pageState.cycleId,
  );
  const streamEvents = filterStreamEventsByPhase(
    allStreamEvents,
    phaseFilter.filterPhaseSeq,
  );
  const auditEvents = filterAuditEventsByPhase(
    allAuditEvents,
    phaseFilter.filterPhaseSeq,
  );

  return (
    <section className="panel task-detail-panel task-attempt-detail task-detail-content--enter">
      <AttemptDetailNavigation taskId={pageState.taskId} />
      <AttemptDetailHeader cycle={cycle} timelineDisplay={timelineDisplay} />
      <AttemptPhasesSection
        taskId={pageState.taskId}
        cycleId={pageState.cycleId}
        cycle={cycle}
        timelineDisplay={timelineDisplay}
        filterPhaseSeq={phaseFilter.filterPhaseSeq}
        onSelectPhase={phaseFilter.setFilterPhaseSeq}
        phaseFilterEnabled={timelineDisplay.showPhaseBadge}
      />
      <AttemptActivitySection
        pageState={pageState}
        cycle={cycle}
        streamEvents={streamEvents}
        allStreamCount={allStreamEvents.length}
        auditEvents={auditEvents}
        allAuditCount={allAuditEvents.length}
        showPhaseBadge={timelineDisplay.showPhaseBadge}
        filterPhaseSeq={phaseFilter.filterPhaseSeq}
        onClearPhaseFilter={phaseFilter.clearFilter}
      />
    </section>
  );
}

function AttemptDetailNavigation({ taskId }: { taskId: string }) {
  return (
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
  );
}

function AttemptDetailHeader({
  cycle,
  timelineDisplay,
}: {
  cycle: TaskCycleDetail;
  timelineDisplay: AttemptTimelineDisplay;
}) {
  return (
    <header className="task-attempt-header">
      <div className="task-attempt-title-group">
        <div className="task-attempt-title-row">
          <h2 className="task-detail-title">Attempt #{cycle.attempt_seq}</h2>
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
            {timelineDisplay.startedParts.date} at {timelineDisplay.startedParts.time}
          </time>
          <span className="task-attempt-meta-inline-item">
            {timelineDisplay.durationLabel}
          </span>
        </p>
      </div>
    </header>
  );
}

function AttemptPhasesSection({
  taskId,
  cycleId,
  cycle,
  timelineDisplay,
  filterPhaseSeq,
  onSelectPhase,
  phaseFilterEnabled,
}: {
  taskId: string;
  cycleId: string;
  cycle: TaskCycleDetail;
  timelineDisplay: AttemptTimelineDisplay;
  filterPhaseSeq: number | null;
  onSelectPhase: (seq: number | null) => void;
  phaseFilterEnabled: boolean;
}) {
  const {
    showPhaseBadge,
    showEndcap,
    endcapLabel,
    endcapTime,
    showStartCap,
    startCapTime,
  } = timelineDisplay;

  return (
    <section
      className="task-attempt-section task-attempt-section--phases"
      aria-labelledby="attempt-phases"
    >
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
            <AttemptPhaseStep
              key={phase.id}
              taskId={taskId}
              cycleId={cycleId}
              phase={phase}
              index={index}
              phaseCount={cycle.phases.length}
              showPhaseBadge={showPhaseBadge}
              showEndcap={showEndcap}
              filterActive={filterPhaseSeq === phase.phase_seq}
              phaseFilterEnabled={phaseFilterEnabled}
              onSelectPhase={() => onSelectPhase(phase.phase_seq)}
            />
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
  );
}

function AttemptPhaseStep({
  taskId,
  cycleId,
  phase,
  index,
  phaseCount,
  showPhaseBadge,
  showEndcap,
  filterActive,
  phaseFilterEnabled,
  onSelectPhase,
}: {
  taskId: string;
  cycleId: string;
  phase: TaskCyclePhase;
  index: number;
  phaseCount: number;
  showPhaseBadge: boolean;
  showEndcap: boolean;
  filterActive: boolean;
  phaseFilterEnabled: boolean;
  onSelectPhase: () => void;
}) {
  const stepClass = [
    "task-attempt-phase-step",
    filterActive && "task-attempt-phase-step--filter-active",
  ]
    .filter(Boolean)
    .join(" ");
  const main = (
    <div className="task-attempt-phase-step-main">
      <span className="task-attempt-phase-step-name">
        {phaseLabel(phase.phase)}
      </span>
      <span className={`cell-pill ${phaseStatusFillClass(phase.status)}`}>
        {phaseStatusLabel(phase.status)}
      </span>
      {showPhaseBadge ? <PhaseSeqBadge seq={phase.phase_seq} /> : null}
    </div>
  );

  return (
    <li
      className={stepClass}
      data-status={phase.status}
      data-last={
        !showEndcap && index === phaseCount - 1 ? "true" : undefined
      }
    >
      <span className="task-attempt-phase-step-marker" aria-hidden="true" />
      {phaseFilterEnabled ? (
        <button
          type="button"
          className="task-attempt-phase-step-button"
          aria-current={filterActive ? "true" : undefined}
          aria-label={`Filter activity to ${phaseLabel(phase.phase)} phase ${phase.phase_seq}`}
          onClick={onSelectPhase}
        >
          {main}
        </button>
      ) : (
        main
      )}
      <PhaseDebugDetails phase={phase} />
      <LivePhaseTail taskId={taskId} cycleId={cycleId} phase={phase} />
    </li>
  );
}

function AttemptActivitySection({
  pageState,
  cycle,
  streamEvents,
  allStreamCount,
  auditEvents,
  allAuditCount,
  showPhaseBadge,
  filterPhaseSeq,
  onClearPhaseFilter,
}: {
  pageState: TaskCycleDetailPageState;
  cycle: TaskCycleDetail;
  streamEvents: TaskCycleStreamEvent[];
  allStreamCount: number;
  auditEvents: NonNullable<
    Awaited<ReturnType<typeof listTaskEvents>>["events"]
  >;
  allAuditCount: number;
  showPhaseBadge: boolean;
  filterPhaseSeq: number | null;
  onClearPhaseFilter: () => void;
}) {
  const {
    activityTab,
    setActivityTab,
    cursorTabId,
    auditTabId,
    cursorPanelId,
    auditPanelId,
    visibleStreamCount,
    setVisibleStreamCount,
    visibleAuditCount,
    setVisibleAuditCount,
    streamQuery,
    taskId,
  } = pageState;

  useEffect(() => {
    setVisibleStreamCount(STREAM_VISIBLE_INITIAL);
    setVisibleAuditCount(AUDIT_VISIBLE_INITIAL);
  }, [filterPhaseSeq, setVisibleStreamCount, setVisibleAuditCount]);

  const filteredPhase = filterPhaseSeq
    ? cycle.phases.find((p) => p.phase_seq === filterPhaseSeq)
    : undefined;
  const filterLabel =
    filteredPhase && filterPhaseSeq
      ? `${phaseLabel(filteredPhase.phase)} #${filterPhaseSeq}`
      : null;
  const streamCountCaption = activityCountCaption(streamEvents.length, allStreamCount);
  const auditCountCaption = activityCountCaption(auditEvents.length, allAuditCount);
  const visibleStreamEvents = streamEvents.slice(0, visibleStreamCount);
  const visibleAuditEvents = auditEvents.slice(0, visibleAuditCount);

  return (
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
            title={streamCountCaption}
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
            title={auditCountCaption}
          >
            Audit
            <span className="task-attempt-activity-tab-count">
              {auditEvents.length}
            </span>
          </button>
        </div>
      </div>

      {filterLabel ? (
        <div className="task-attempt-activity-filter-bar">
          <p className="task-attempt-activity-filter-label">
            Showing {filterLabel}
          </p>
          <button
            type="button"
            className="secondary task-attempt-activity-filter-clear"
            onClick={onClearPhaseFilter}
          >
            Clear filter
          </button>
        </div>
      ) : null}

      {activityTab === "cursor" ? (
        <CursorActivityPanel
          panelId={cursorPanelId}
          tabId={cursorTabId}
          streamQuery={streamQuery}
          streamEvents={streamEvents}
          visibleStreamEvents={visibleStreamEvents}
          showPhaseBadge={showPhaseBadge}
          filterLabel={filterLabel}
          onClearPhaseFilter={onClearPhaseFilter}
          onLoadMore={() => setVisibleStreamCount((n) => n + LOAD_MORE_STEP)}
        />
      ) : (
        <AuditActivityPanel
          panelId={auditPanelId}
          tabId={auditTabId}
          auditQuery={pageState.auditQuery}
          auditEvents={auditEvents}
          visibleAuditEvents={visibleAuditEvents}
          taskId={taskId}
          filterLabel={filterLabel}
          onClearPhaseFilter={onClearPhaseFilter}
          onLoadMore={() => setVisibleAuditCount((n) => n + LOAD_MORE_STEP)}
        />
      )}
    </section>
  );
}

function CursorActivityPanel({
  panelId,
  tabId,
  streamQuery,
  streamEvents,
  visibleStreamEvents,
  showPhaseBadge,
  filterLabel,
  onClearPhaseFilter,
  onLoadMore,
}: {
  panelId: string;
  tabId: string;
  streamQuery: UseTaskCycleStreamResult;
  streamEvents: TaskCycleStreamEvent[];
  visibleStreamEvents: TaskCycleStreamEvent[];
  showPhaseBadge: boolean;
  filterLabel: string | null;
  onClearPhaseFilter: () => void;
  onLoadMore: () => void;
}) {
  return (
    <div
      role="tabpanel"
      id={panelId}
      aria-labelledby={tabId}
      className="task-attempt-activity-panel"
    >
      {streamQuery.isError ? (
        <div className="err" role="alert">
          <p>
            {errorMessage(streamQuery.error, "Could not load stream events.")}
          </p>
        </div>
      ) : streamEvents.length === 0 ? (
        filterLabel ? (
          <EmptyState
            title={`No Cursor output for ${filterLabel}`}
            description="Try another phase or show all activity."
            density="compact"
            hideIcon
            action={{ label: "Show all phases", onClick: onClearPhaseFilter }}
          />
        ) : (
          <EmptyState
            title="No Cursor output yet"
            description="Stream lines appear here as the agent runs."
            density="compact"
            hideIcon
          />
        )
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
            onLoadMore={onLoadMore}
          />
        </>
      )}
    </div>
  );
}

function AuditActivityPanel({
  panelId,
  tabId,
  auditQuery,
  auditEvents,
  visibleAuditEvents,
  taskId,
  filterLabel,
  onClearPhaseFilter,
  onLoadMore,
}: {
  panelId: string;
  tabId: string;
  auditQuery: TaskCycleDetailPageState["auditQuery"];
  auditEvents: NonNullable<
    Awaited<ReturnType<typeof listTaskEvents>>["events"]
  >;
  visibleAuditEvents: NonNullable<
    Awaited<ReturnType<typeof listTaskEvents>>["events"]
  >;
  taskId: string;
  filterLabel: string | null;
  onClearPhaseFilter: () => void;
  onLoadMore: () => void;
}) {
  return (
    <div
      role="tabpanel"
      id={panelId}
      aria-labelledby={tabId}
      className="task-attempt-activity-panel"
    >
      {auditQuery.isPending ? (
        <TaskTimelineSkeleton />
      ) : auditQuery.isError ? (
        <div className="err" role="alert">
          <p>{errorMessage(auditQuery.error, "Could not load audit events.")}</p>
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
        filterLabel ? (
          <EmptyState
            title={`No audit events for ${filterLabel}`}
            description="Try another phase or show all activity."
            density="compact"
            hideIcon
            action={{ label: "Show all phases", onClick: onClearPhaseFilter }}
          />
        ) : (
          <EmptyState
            title="No audit events yet"
            description="System events for this attempt appear here."
            density="compact"
            hideIcon
          />
        )
      ) : (
        <>
          <AttemptAuditTimeline
            events={visibleAuditEvents}
            taskId={taskId}
            ariaLabelledBy={tabId}
          />
          <LoadMoreRows
            shown={visibleAuditEvents.length}
            total={auditEvents.length}
            itemLabel="events"
            onLoadMore={onLoadMore}
          />
        </>
      )}
    </div>
  );
}

function sortStreamEventsNewestFirst(
  events: readonly TaskCycleStreamEvent[],
): TaskCycleStreamEvent[] {
  return [...events].sort((a, b) => b.stream_seq - a.stream_seq);
}

function filterAuditEventsForCycle(
  events: Awaited<ReturnType<typeof listTaskEvents>>["events"] | undefined,
  cycleId: string,
) {
  return (events?.filter((ev) => ev.data.cycle_id === cycleId) ?? []).sort(
    (a, b) => b.seq - a.seq,
  );
}

function buildAttemptTimelineDisplay(
  cycle: TaskCycleDetail,
  now: number,
): AttemptTimelineDisplay {
  const showPhaseBadge = cycle.phases.length > 1;
  const endcapLabel = attemptEndcapLabel(cycle.status);
  const showEndcap = endcapLabel !== null && cycle.phases.length > 0;
  const endcapTime = showEndcap ? formatAttemptEndedTime(cycle.ended_at) : null;
  const showStartCap = cycle.phases.length > 0;
  const startCapTime = showStartCap
    ? formatAttemptEndedTime(cycle.started_at)
    : null;

  return {
    startedParts: formatAttemptStartedParts(cycle.started_at),
    durationLabel: formatAttemptDurationMeta(
      cycle.started_at,
      cycle.ended_at,
      cycle.status,
      now,
    ),
    showPhaseBadge,
    endcapLabel,
    showEndcap,
    endcapTime,
    showStartCap,
    startCapTime,
  };
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
  return (
    <div className="task-attempt-live-tail" aria-live="polite">
      <div className="task-attempt-live-tail-heading">
        <span className="cycle-live-dot task-attempt-live-dot" aria-hidden="true" />
        <span>Live</span>
      </div>
      <CycleLiveProgressList
        items={live}
        now={now}
        listAriaLabel="Recent live updates"
        timestampMode="clock"
        showPendingRow
      />
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
  const kind = agentProgressKindDescriptor(ev.kind, ev.subtype, ev.tool);
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
