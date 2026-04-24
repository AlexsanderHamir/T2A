import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { ObservabilityCommandCenter } from "./ObservabilityCommandCenter";
import { ObservabilityCycles } from "./ObservabilityCycles";
import { ObservabilityLogs } from "./ObservabilityLogs";
import { ObservabilityOverview } from "./ObservabilityOverview";
import { ObservabilityRunnerBreakdown } from "./ObservabilityRunnerBreakdown";
import { ObservabilitySystem } from "./ObservabilitySystem";
import { useObservabilityStats } from "./useObservabilityStats";
import { useSystemHealth } from "./useSystemHealth";

/**
 * Top-level observability route. The page leads with a small command
 * summary, then moves from execution diagnostics into supporting
 * distributions and runtime details.
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
          Operator view of task execution, agent health, and runtime pressure.
          The page keeps the signal first and the raw detail close behind.
        </p>
      </header>
      <ObservabilityCommandCenter
        stats={stats}
        statsLoading={loading}
        health={health}
        healthLoading={healthLoading}
      />
      <div className="obs-page-diagnostics">
        <ObservabilityRunnerBreakdown stats={stats} loading={loading} />
        <ObservabilityCycles stats={stats} loading={loading} />
      </div>
      <div className="obs-page-supporting">
        <ObservabilityOverview stats={stats} loading={loading} />
        <ObservabilitySystem health={health} loading={healthLoading} />
      </div>
      <ObservabilityLogs />
    </div>
  );
}
