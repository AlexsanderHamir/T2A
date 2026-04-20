import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { ObservabilityCycles } from "./ObservabilityCycles";
import { ObservabilityOverview } from "./ObservabilityOverview";
import { useObservabilityStats } from "./useObservabilityStats";

/**
 * Top-level observability route. Composes the Overview (KPI counters
 * and distribution charts) with the Cycles & Phases section (heatmap
 * and recent failures). Both panes share the same `useObservabilityStats`
 * snapshot so they stay perfectly in sync, and the SSE invalidation
 * wired in `useTaskEventStream` keeps the whole page live without
 * extra polling.
 */
export function ObservabilityPage() {
  useDocumentTitle("Observability");
  const { stats, loading } = useObservabilityStats();
  return (
    <div className="obs-page task-detail-content--enter">
      <header className="obs-page-header">
        <h2 className="obs-page-title">Observability</h2>
        <p className="obs-page-subtitle">
          Live snapshot of the task table — refreshes automatically as the
          agent worker progresses through tasks.
        </p>
      </header>
      <ObservabilityOverview stats={stats} loading={loading} />
      <ObservabilityCycles stats={stats} loading={loading} />
    </div>
  );
}
