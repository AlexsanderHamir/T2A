import type { Phase, PhaseStatus } from "@/types/cycle";
import type { TaskStatsResponse } from "@/types/task";
import {
  PHASE_DISPLAY_ORDER,
  PHASE_STATUS_DISPLAY_ORDER,
  phaseCellCount,
  phaseCellIntensity,
  phaseLabel,
  phaseStatusFillClass,
  phaseStatusLabel,
  totalPhaseCount,
} from "./cyclesViewModel";

type Props = {
  stats: TaskStatsResponse;
};

/**
 * 4×4 heatmap showing the count of cycle phases broken down by
 * (phase, status). Colour intensity is normalised against the
 * brightest cell so a single hot cell stands out even when the rest of
 * the grid is sparse. Empty cells render at neutral intensity (still
 * visible) so the grid keeps its shape.
 *
 * The whole component renders in two states:
 *   - empty: no phases recorded → caption surfaces the empty state and
 *     every cell shows `0` at neutral fill.
 *   - populated: caption shows the total phase count; cell tooltip
 *     carries phase + status + count for hover precision.
 */
export function PhaseHeatmap({ stats }: Props) {
  const total = totalPhaseCount(stats);
  const hasData = total > 0;
  return (
    <section className="obs-heatmap" aria-label="Phase × status heatmap">
      <header className="obs-heatmap-head">
        <h3 className="obs-heatmap-title">Phase × status</h3>
        <p className="obs-heatmap-caption">
          {hasData
            ? `${total} phase ${total === 1 ? "row" : "rows"} recorded`
            : "No phases have run yet"}
        </p>
      </header>
      <div
        className="obs-heatmap-grid"
        role="table"
        aria-label="Phase by status counts"
      >
        <div className="obs-heatmap-row obs-heatmap-row--head" role="row">
          <span className="obs-heatmap-cell obs-heatmap-cell--corner" role="columnheader" />
          {PHASE_STATUS_DISPLAY_ORDER.map((status) => (
            <span
              key={status}
              className="obs-heatmap-cell obs-heatmap-cell--colhead"
              role="columnheader"
            >
              {phaseStatusLabel(status)}
            </span>
          ))}
        </div>
        {PHASE_DISPLAY_ORDER.map((phase) => (
          <PhaseRow key={phase} stats={stats} phase={phase} />
        ))}
      </div>
    </section>
  );
}

function PhaseRow({ stats, phase }: { stats: TaskStatsResponse; phase: Phase }) {
  return (
    <div className="obs-heatmap-row" role="row">
      <span
        className="obs-heatmap-cell obs-heatmap-cell--rowhead"
        role="rowheader"
      >
        {phaseLabel(phase)}
      </span>
      {PHASE_STATUS_DISPLAY_ORDER.map((status) => (
        <PhaseCell key={status} stats={stats} phase={phase} status={status} />
      ))}
    </div>
  );
}

function PhaseCell({
  stats,
  phase,
  status,
}: {
  stats: TaskStatsResponse;
  phase: Phase;
  status: PhaseStatus;
}) {
  const count = phaseCellCount(stats, phase, status);
  const intensity = phaseCellIntensity(stats, phase, status);
  const fillClass = count > 0 ? phaseStatusFillClass(status) : "";
  const className = ["obs-heatmap-cell", "obs-heatmap-cell--data", fillClass]
    .filter(Boolean)
    .join(" ");
  const phaseName = phaseLabel(phase);
  const statusName = phaseStatusLabel(status);
  return (
    <span
      className={className}
      role="cell"
      title={`${phaseName} ${statusName}: ${count}`}
      aria-label={`${phaseName} ${statusName} ${count}`}
      data-testid={`obs-heatmap-cell-${phase}-${status}`}
      style={{ "--cell-intensity": intensity } as React.CSSProperties}
    >
      {count}
    </span>
  );
}
