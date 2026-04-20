import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { ObservabilityCycles } from "./ObservabilityCycles";
import { ObservabilityOverview } from "./ObservabilityOverview";
import { ObservabilityRunnerBreakdown } from "./ObservabilityRunnerBreakdown";
import { ObservabilitySystem } from "./ObservabilitySystem";
import { useObservabilityStats } from "./useObservabilityStats";
import { useSystemHealth } from "./useSystemHealth";

/**
 * Top-level observability route. Composes three scrollable sections,
 * each backed by its own data source so they refresh independently
 * without cross-talk:
 *
 *   1. Overview — KPI counters and distribution charts, fed by
 *      `["task-stats"]` and live-updated through `useTaskEventStream`.
 *   2. Cycles & phases — heatmap and recent failures, same source.
 *   3. System health — operator-facing snapshot of the running
 *      process, fed by `GET /system/health`. Polls on a fixed interval
 *      because no SSE event corresponds to "system metrics changed."
 */
export function ObservabilityPage() {
  useDocumentTitle("Observability");
  const { stats, loading } = useObservabilityStats();
  const { health, loading: healthLoading } = useSystemHealth();
  return (
    <div className="obs-page task-detail-content--enter">
      <header className="obs-page-header">
        <h2 className="obs-page-title">Observability</h2>
        <p className="obs-page-subtitle">
          Live snapshot of the task table and the running process —
          refreshes automatically as the agent worker progresses.
        </p>
      </header>
      <ObservabilityOverview stats={stats} loading={loading} />
      <ObservabilityRunnerBreakdown stats={stats} loading={loading} />
      <ObservabilityCycles stats={stats} loading={loading} />
      <ObservabilitySystem health={health} loading={healthLoading} />
    </div>
  );
}
