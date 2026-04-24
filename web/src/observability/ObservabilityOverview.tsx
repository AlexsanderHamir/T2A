import type { TaskStatsResponse } from "@/types/task";
import { Donut } from "./Donut";
import { StackedBar } from "./StackedBar";
import {
  PRIORITY_DISPLAY_ORDER,
  STATUS_DISPLAY_ORDER,
  doneCount,
  priorityFillClass,
  priorityLabel,
  statusFillClass,
  statusLabel,
} from "./statsViewModel";

type Props = {
  stats: TaskStatsResponse | null | undefined;
  loading: boolean;
};

/**
 * Supporting task inventory and distribution charts. Headline operational
 * counters live in ObservabilityCommandCenter; this pane keeps the broader
 * shape of the task table available without competing for first glance.
 */
export function ObservabilityOverview({ stats, loading }: Props) {
  const hasStats = stats != null;

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
    <section className="obs-overview" aria-label="Task inventory and distributions">
      <header className="obs-section-head">
        <div>
          <p className="obs-section-kicker">Task telemetry</p>
          <h3 className="obs-section-title">Work distribution</h3>
        </div>
        <p className="obs-section-caption">{inventoryCaption(stats, loading)}</p>
      </header>

      <section className="obs-inventory-grid" aria-label="Task inventory">
        <InventoryMetric
          label="Total"
          value={stats?.total}
          loading={loading}
          available={hasStats}
          meta={totalMeta(stats, loading)}
          testId="obs-inventory-total"
        />
        <InventoryMetric
          label="Completed"
          value={stats ? doneCount(stats) : undefined}
          loading={loading}
          available={hasStats}
          meta="done tasks"
          testId="obs-inventory-done"
        />
        <InventoryMetric
          label="Scheduled"
          value={stats?.scheduled}
          loading={loading}
          available={hasStats}
          meta="deferred pickup"
          testId="obs-inventory-scheduled"
        />
        <InventoryMetric
          label="Critical"
          value={stats?.by_priority.critical ?? stats?.critical}
          loading={loading}
          available={hasStats}
          meta="critical priority"
          testId="obs-inventory-critical"
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
    </section>
  );
}

function InventoryMetric({
  label,
  value,
  loading,
  available,
  meta,
  testId,
}: {
  label: string;
  value: number | undefined;
  loading: boolean;
  available: boolean;
  meta: string;
  testId: string;
}) {
  const content = loading && !available ? "Loading" : available ? String(value ?? 0) : "—";
  return (
    <article
      className="obs-inventory-card"
      aria-busy={loading && !available}
      data-testid={testId}
    >
      <p className="obs-inventory-label">{label}</p>
      <p className="obs-inventory-value">{content}</p>
      <p className="obs-inventory-meta">{meta}</p>
    </article>
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

function inventoryCaption(
  stats: TaskStatsResponse | null | undefined,
  loading: boolean,
): string {
  if (!stats) return loading ? "Loading the task table shape…" : "Task inventory unavailable.";
  if (stats.total === 0) return "No tasks recorded yet.";
  return `${stats.total} task${stats.total === 1 ? "" : "s"} across status, priority, and scope.`;
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
