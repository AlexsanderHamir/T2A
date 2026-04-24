import type { SystemHealthResponse } from "@/types";
import type { TaskStatsResponse } from "@/types/task";
import { KpiCard } from "./KpiCard";
import { kpiState } from "./kpiState";
import { blockedCount, failedCount, reviewCount, runningCount } from "./statsViewModel";
import { formatNumber, formatRatio, summarize } from "./systemHealthViewModel";

type Props = {
  stats: TaskStatsResponse | null | undefined;
  statsLoading: boolean;
  health: SystemHealthResponse | null | undefined;
  healthLoading: boolean;
};

export function ObservabilityCommandCenter({
  stats,
  statsLoading,
  health,
  healthLoading,
}: Props) {
  const summary = summarize(health, healthLoading);
  const hasStats = stats != null;
  const hasHealth = health != null;
  const failedTasks = stats ? failedCount(stats) : undefined;
  const activeWork = stats
    ? runningCount(stats) + blockedCount(stats) + reviewCount(stats)
    : undefined;
  const readyNow = stats?.by_status.ready ?? stats?.ready;
  const queueDepth = health?.agent.queue_depth;
  const droppedFrames = health?.sse.dropped_frames_total;

  return (
    <section
      className={`obs-command obs-command--${summary.level}`}
      aria-label="Operational summary"
    >
      <div className="obs-command-copy">
        <div className="obs-command-kicker">
          <span className={`obs-command-status obs-command-status--${summary.level}`}>
            {statusLabel(summary.level)}
          </span>
          <span className="obs-command-refresh">Live data refreshes automatically</span>
        </div>
        <h3 className="obs-command-title">{headline(stats, statsLoading, health, healthLoading)}</h3>
        <p className="obs-command-caption">{summary.caption}</p>
      </div>

      <div className="obs-command-grid" aria-label="Priority metrics">
        <KpiCard
          label="Failed tasks"
          state={kpiState(failedTasks, statsLoading, hasStats)}
          meta={failedTaskMeta(stats, statsLoading)}
          tone={(failedTasks ?? 0) > 0 ? "danger" : "positive"}
          testId="obs-command-failed"
        />
        <KpiCard
          label="Active work"
          state={kpiState(activeWork, statsLoading, hasStats)}
          meta={activeWorkMeta(stats, statsLoading)}
          tone={(blockedCountOrZero(stats) + reviewCountOrZero(stats)) > 0 ? "warning" : "info"}
          testId="obs-command-active"
        />
        <KpiCard
          label="Ready now"
          state={kpiState(readyNow, statsLoading, hasStats)}
          meta={readyMeta(stats, statsLoading)}
          tone="info"
          testId="obs-command-ready"
        />
        <KpiCard
          label="Agent queue"
          state={kpiState(queueDepth, healthLoading, hasHealth)}
          meta={queueMeta(health, healthLoading)}
          tone={queueTone(health)}
          testId="obs-command-queue"
        />
        <KpiCard
          label="Dropped SSE frames"
          state={kpiState(droppedFrames, healthLoading, hasHealth)}
          meta="slow-client backpressure"
          tone={(droppedFrames ?? 0) > 0 ? "danger" : "positive"}
          testId="obs-command-dropped"
        />
      </div>
    </section>
  );
}

function statusLabel(level: ReturnType<typeof summarize>["level"]): string {
  switch (level) {
    case "ok":
      return "Healthy";
    case "paused":
      return "Paused";
    case "degraded":
      return "Needs attention";
    case "unknown":
      return "Checking";
  }
}

function headline(
  stats: TaskStatsResponse | null | undefined,
  statsLoading: boolean,
  health: SystemHealthResponse | null | undefined,
  healthLoading: boolean,
): string {
  if ((statsLoading && !stats) || (healthLoading && !health)) {
    return "Collecting task and runtime signals.";
  }
  if (!stats && !health) {
    return "Telemetry is unavailable right now.";
  }

  const failures = stats ? failedCount(stats) : 0;
  const blocked = stats ? blockedCount(stats) : 0;
  const dropped = health?.sse.dropped_frames_total ?? 0;
  const failedRuns = health?.agent.runs_by_terminal_status.failed ?? 0;
  const parts: string[] = [];
  if (failures > 0) parts.push(`${formatNumber(failures)} failed task${failures === 1 ? "" : "s"}`);
  if (blocked > 0) parts.push(`${formatNumber(blocked)} blocked task${blocked === 1 ? "" : "s"}`);
  if (failedRuns > 0) {
    parts.push(`${formatNumber(failedRuns)} failed agent run${failedRuns === 1 ? "" : "s"}`);
  }
  if (dropped > 0) parts.push(`${formatNumber(dropped)} dropped event frame${dropped === 1 ? "" : "s"}`);
  if (parts.length > 0) return `${parts.join(" · ")} need attention.`;
  return "No active incident signals across tasks and runtime.";
}

function failedTaskMeta(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
): string {
  if (!stats) return loading ? "checking task outcomes" : "task stats unavailable";
  return `${formatNumber(stats.total)} total task${stats.total === 1 ? "" : "s"}`;
}

function activeWorkMeta(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
): string {
  if (!stats) return loading ? "checking active states" : "active states unavailable";
  return `${runningCount(stats)} running · ${blockedCount(stats)} blocked · ${reviewCount(stats)} review`;
}

function readyMeta(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
): string {
  if (!stats) return loading ? "checking pickup queue" : "pickup queue unavailable";
  return `${formatNumber(stats.scheduled)} scheduled for later`;
}

function queueMeta(
  health: SystemHealthResponse | null | undefined,
  loading: boolean,
): string {
  if (!health) return loading ? "checking worker capacity" : "worker capacity unavailable";
  return `${formatRatio(health.agent.queue_depth, health.agent.queue_capacity)} pending`;
}

function queueTone(health: SystemHealthResponse | null | undefined) {
  if (!health) return "info";
  if (
    health.agent.queue_capacity > 0 &&
    health.agent.queue_depth >= health.agent.queue_capacity
  ) {
    return "warning";
  }
  return "info";
}

function blockedCountOrZero(stats: TaskStatsResponse | null | undefined): number {
  return stats ? blockedCount(stats) : 0;
}

function reviewCountOrZero(stats: TaskStatsResponse | null | undefined): number {
  return stats ? reviewCount(stats) : 0;
}
