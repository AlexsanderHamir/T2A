import type { TaskStatsResponse } from "@/types/task";
import {
  CYCLE_STATUS_DISPLAY_ORDER,
  cycleStatusFillClass,
  cycleStatusLabel,
  totalCycleCount,
} from "./cyclesViewModel";
import { PhaseHeatmap } from "./PhaseHeatmap";
import { RecentFailuresTable } from "./RecentFailuresTable";
import { StackedBar } from "./StackedBar";

type Props = {
  stats: TaskStatsResponse | null | undefined;
  loading: boolean;
};

/**
 * Cycles & phases pane. Sits below the Overview on the Observability
 * page. Three sub-sections:
 *
 *  1. Cycle outcome bar — succeeded / failed / aborted / running mix.
 *  2. Phase × status heatmap — lets the operator see where work
 *     concentrates, and (more importantly) where failures concentrate
 *     ("the failed-in-failed-stage view" the user originally asked
 *     for).
 *  3. Recent failures table — newest 25 cycle_failed events with
 *     deep links to the originating audit row.
 *
 * All three render reasonable empty/loading states so the section is
 * non-empty even on a fresh database.
 */
export function ObservabilityCycles({ stats, loading }: Props) {
  if (!stats) {
    return (
      <section className="obs-cycles" aria-label="Cycles & phases">
        <header className="obs-cycles-head">
          <h3 className="obs-cycles-title">Cycles &amp; phases</h3>
          <p className="obs-cycles-subtitle">
            {loading
              ? "Loading cycle telemetry…"
              : "Cycle telemetry unavailable."}
          </p>
        </header>
      </section>
    );
  }

  const totalCycles = totalCycleCount(stats);
  const cycleSegments = CYCLE_STATUS_DISPLAY_ORDER.map((status) => ({
    id: status,
    label: cycleStatusLabel(status),
    value: stats.cycles.by_status[status] ?? 0,
    fillClass: cycleStatusFillClass(status),
  }));

  return (
    <section className="obs-cycles" aria-label="Cycles & phases">
      <header className="obs-cycles-head">
        <h3 className="obs-cycles-title">Cycles &amp; phases</h3>
        <p className="obs-cycles-subtitle">
          {totalCycles === 0
            ? "No execution cycles recorded yet — start a task to populate this view."
            : `${totalCycles} cycle ${totalCycles === 1 ? "attempt" : "attempts"} across all tasks.`}
        </p>
      </header>
      <div className="obs-cycles-grid">
        <StackedBar
          title="Cycle outcomes"
          segments={cycleSegments}
          caption={
            totalCycles === 0
              ? "No cycle data yet"
              : `${totalCycles} total`
          }
        />
        <PhaseHeatmap stats={stats} />
      </div>
      <RecentFailuresTable
        failures={stats.recent_failures}
        titleHref="/observability/failures"
      />
    </section>
  );
}
