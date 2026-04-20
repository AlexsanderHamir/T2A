import type { TaskStatsResponse } from "@/types/task";
import { Donut } from "./Donut";
import { KpiCard } from "./KpiCard";
import { kpiState } from "./kpiState";
import { StackedBar } from "./StackedBar";
import {
  PRIORITY_DISPLAY_ORDER,
  STATUS_DISPLAY_ORDER,
  blockedCount,
  doneCount,
  failedCount,
  priorityFillClass,
  priorityLabel,
  reviewCount,
  runningCount,
  statusFillClass,
  statusLabel,
} from "./statsViewModel";

type Props = {
  stats: TaskStatsResponse | null | undefined;
  loading: boolean;
};

/**
 * Stage-1 observability overview built entirely from the existing
 * `GET /tasks/stats` payload — no backend change. The page exposes the
 * five things an operator most often asks the dashboard:
 *   1. How many tasks total, and what's the parent/subtask split?
 *   2. How many are done vs failed vs in-flight right now?
 *   3. What does the status distribution look like across the table?
 *   4. What does the priority distribution look like?
 *   5. (Implicit via #3) where are tasks getting stuck?
 *
 * Live updates ride the existing `["task-stats"]` invalidation in
 * `useTaskEventStream` so cycle frames refresh this view automatically.
 */
export function ObservabilityOverview({ stats, loading }: Props) {
  const hasStats = stats != null;

  const totalState = kpiState(stats?.total, loading, hasStats);
  const doneState = kpiState(
    stats ? doneCount(stats) : undefined,
    loading,
    hasStats,
  );
  const failedState = kpiState(
    stats ? failedCount(stats) : undefined,
    loading,
    hasStats,
  );
  const runningState = kpiState(
    stats ? runningCount(stats) : undefined,
    loading,
    hasStats,
  );
  const blockedState = kpiState(
    stats ? blockedCount(stats) : undefined,
    loading,
    hasStats,
  );
  const reviewState = kpiState(
    stats ? reviewCount(stats) : undefined,
    loading,
    hasStats,
  );
  const readyState = kpiState(
    stats?.by_status.ready ?? stats?.ready,
    loading,
    hasStats,
  );
  // Stage 6 KPI: tasks intentionally deferred via
  // `pickup_not_before > now()`. The label "Scheduled (deferred)"
  // is exactly the plan's wording so the operator's mental model
  // links to the same word that appears on the task list filter
  // dropdown ("Scheduled (deferred)"). Distinguishing this from
  // "Ready" is the whole point: "0 ready, 12 scheduled" is a
  // perfectly healthy state ("operator told the agent to wait")
  // whereas "0 ready, 0 scheduled" with a paused agent is the
  // genuinely-stuck state.
  const scheduledState = kpiState(stats?.scheduled, loading, hasStats);
  const criticalState = kpiState(
    stats?.by_priority.critical ?? stats?.critical,
    loading,
    hasStats,
  );

  const statusSegments = STATUS_DISPLAY_ORDER.map((s) => ({
    id: s,
    label: statusLabel(s),
    value: stats?.by_status[s] ?? 0,
    fillClass: statusFillClass(s),
  }));

  const prioritySegments = PRIORITY_DISPLAY_ORDER.map((p) => ({
    id: p,
    label: priorityLabel(p),
    value: stats?.by_priority[p] ?? 0,
    fillClass: priorityFillClass(p),
  }));

  const scopeSlices = [
    {
      id: "parent",
      label: "Parent",
      value: stats?.by_scope.parent ?? 0,
      fillClass: "obs-donut-arc--parent",
    },
    {
      id: "subtask",
      label: "Subtask",
      value: stats?.by_scope.subtask ?? 0,
      fillClass: "obs-donut-arc--subtask",
    },
  ];

  return (
    <div className="obs-overview">
      <section className="obs-kpi-grid" aria-label="Headline counters">
        <KpiCard
          label="Total tasks"
          state={totalState}
          meta={totalMeta(stats, loading)}
          tone="info"
          testId="obs-kpi-total"
        />
        <KpiCard
          label="Done"
          state={doneState}
          meta="completed tasks"
          tone="positive"
          testId="obs-kpi-done"
        />
        <KpiCard
          label="Failed"
          state={failedState}
          meta="needs investigation"
          tone="danger"
          testId="obs-kpi-failed"
        />
        <KpiCard
          label="Running"
          state={runningState}
          meta="agent actively executing"
          tone="info"
          testId="obs-kpi-running"
        />
        <KpiCard
          label="Blocked"
          state={blockedState}
          meta="waiting on a dependency"
          tone="warning"
          testId="obs-kpi-blocked"
        />
        <KpiCard
          label="In review"
          state={reviewState}
          meta="awaiting human approval"
          tone="warning"
          testId="obs-kpi-review"
        />
        <KpiCard
          label="Ready"
          state={readyState}
          meta="ready for agent pickup"
          tone="info"
          testId="obs-kpi-ready"
        />
        <KpiCard
          label="Scheduled (deferred)"
          state={scheduledState}
          meta="queued for a future time"
          tone="info"
          testId="obs-kpi-scheduled"
        />
        <KpiCard
          label="Critical"
          state={criticalState}
          meta="critical priority"
          tone="danger"
          testId="obs-kpi-critical"
        />
      </section>

      <section className="obs-charts" aria-label="Distribution charts">
        <StackedBar
          title="Status distribution"
          segments={statusSegments}
          caption={distributionCaption(stats, loading, "by_status")}
        />
        <StackedBar
          title="Priority distribution"
          segments={prioritySegments}
          caption={distributionCaption(stats, loading, "by_priority")}
        />
        <Donut
          title="Scope"
          slices={scopeSlices}
          caption={scopeCaption(stats, loading)}
        />
      </section>
    </div>
  );
}

function totalMeta(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
): string {
  if (stats == null) {
    return loading ? "Loading breakdown…" : "Breakdown unavailable";
  }
  const p = stats.by_scope.parent;
  const s = stats.by_scope.subtask;
  return `${p} parent • ${s} subtask${s === 1 ? "" : "s"}`;
}

function distributionCaption(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
  source: "by_status" | "by_priority",
): string | undefined {
  if (stats == null) {
    return loading ? "Loading…" : "Unavailable";
  }
  const total = Object.values(stats[source]).reduce(
    (acc, v) => acc + (v ?? 0),
    0,
  );
  if (total === 0) return "No tasks recorded yet";
  return `${total} task${total === 1 ? "" : "s"}`;
}

function scopeCaption(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
): string | undefined {
  if (stats == null) {
    return loading ? "Loading…" : "Unavailable";
  }
  const total = stats.by_scope.parent + stats.by_scope.subtask;
  if (total === 0) return "No tasks recorded yet";
  return `${total} task${total === 1 ? "" : "s"}`;
}
