import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { ObservabilityOverview } from "./ObservabilityOverview";
import { useObservabilityStats } from "./useObservabilityStats";

/**
 * Top-level observability route. Stage 1 ships only the Overview pane;
 * future stages will add Cycles & Phases and System & API tabs as
 * sibling sections inside this shell (see the observability rollout
 * plan). Keeping the shell here from day one means we don't have to
 * rewire `App.tsx` when the additional panes land.
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
    </div>
  );
}
