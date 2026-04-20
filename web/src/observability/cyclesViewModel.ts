/**
 * Display helpers for the Cycles & Phases observability section. Like
 * statsViewModel.ts, this file owns the display order, labels, and
 * CSS-class generators so the React components stay focused on layout.
 */

import type {
  CycleStatus,
  Phase,
  PhaseStatus,
} from "@/types/cycle";
import type { TaskStatsResponse } from "@/types/task";

/**
 * Display order for cycle outcomes — matches the Status bar's
 * "running → terminal-good → terminal-bad" pattern so the colour story
 * stays consistent across the page.
 */
export const CYCLE_STATUS_DISPLAY_ORDER: CycleStatus[] = [
  "running",
  "succeeded",
  "failed",
  "aborted",
];

/**
 * Display order for the heatmap rows — matches the runtime sequence
 * (diagnose → execute → verify → persist) from `domain.Phase`.
 */
export const PHASE_DISPLAY_ORDER: Phase[] = [
  "diagnose",
  "execute",
  "verify",
  "persist",
];

/**
 * Display order for the heatmap columns — matches the lifecycle order
 * defined by `domain.PhaseStatus` (no `aborted`; phases skip rather
 * than abort). `running` first so the eye lands on in-flight cells.
 */
export const PHASE_STATUS_DISPLAY_ORDER: PhaseStatus[] = [
  "running",
  "succeeded",
  "failed",
  "skipped",
];

const CYCLE_STATUS_LABELS: Record<CycleStatus, string> = {
  running: "Running",
  succeeded: "Succeeded",
  failed: "Failed",
  aborted: "Aborted",
};

const PHASE_LABELS: Record<Phase, string> = {
  diagnose: "Diagnose",
  execute: "Execute",
  verify: "Verify",
  persist: "Persist",
};

const PHASE_STATUS_LABELS: Record<PhaseStatus, string> = {
  running: "Running",
  succeeded: "Succeeded",
  failed: "Failed",
  skipped: "Skipped",
};

export function cycleStatusLabel(s: CycleStatus): string {
  return CYCLE_STATUS_LABELS[s];
}

export function phaseLabel(p: Phase): string {
  return PHASE_LABELS[p];
}

export function phaseStatusLabel(s: PhaseStatus): string {
  return PHASE_STATUS_LABELS[s];
}

/**
 * CSS class for a cycle-outcome swatch / segment. CycleStatus shares
 * the same colour family as task Status (`succeeded → done`,
 * `failed → failed`, etc.); we map onto the existing `cell-pill--`
 * classes so the colour story stays uniform.
 */
export function cycleStatusFillClass(s: CycleStatus): string {
  switch (s) {
    case "running":
      return "cell-pill--status-running";
    case "succeeded":
      return "cell-pill--status-done";
    case "failed":
      return "cell-pill--status-failed";
    case "aborted":
      // Aborted intentionally maps to a *dedicated* class rather than
      // aliasing onto status-blocked. Reason: the chart palette
      // overhaul moved blocked to amber to disambiguate the status
      // bar; if aborted kept the alias it would render amber too,
      // making "aborted" visually indistinguishable from a hot
      // blocked task even though semantically it is a quiet,
      // deliberate stop. A dedicated class lets us bind aborted to
      // a neutral stone hue (--obs-fill-cycle-aborted) for both the
      // chart segment and the text pill (see app-task-list-and-mentions.css).
      return "cell-pill--cycle-aborted";
  }
}

/**
 * CSS class for a phase-status swatch / heatmap cell. PhaseStatus
 * mirrors CycleStatus (no aborted); reuse the same colour family.
 */
export function phaseStatusFillClass(s: PhaseStatus): string {
  switch (s) {
    case "running":
      return "cell-pill--status-running";
    case "succeeded":
      return "cell-pill--status-done";
    case "failed":
      return "cell-pill--status-failed";
    case "skipped":
      return "cell-pill--status-blocked";
  }
}

/** Total cycles recorded across all statuses. */
export function totalCycleCount(stats: TaskStatsResponse): number {
  return Object.values(stats.cycles.by_status).reduce(
    (acc, n) => acc + (n ?? 0),
    0,
  );
}

/**
 * Total phase rows across the heatmap — used to size cell intensity.
 * Returning 0 makes every cell render the empty/neutral style.
 */
export function totalPhaseCount(stats: TaskStatsResponse): number {
  let acc = 0;
  for (const phase of PHASE_DISPLAY_ORDER) {
    const inner = stats.phases.by_phase_status[phase];
    for (const status of PHASE_STATUS_DISPLAY_ORDER) {
      acc += inner[status] ?? 0;
    }
  }
  return acc;
}

export function phaseCellCount(
  stats: TaskStatsResponse,
  phase: Phase,
  status: PhaseStatus,
): number {
  return stats.phases.by_phase_status[phase][status] ?? 0;
}

/**
 * Returns a 0..1 intensity for a heatmap cell — the cell's count
 * normalised against the **largest** non-zero cell so the brightest
 * cell renders at full strength. Returns 0 when the heatmap is empty
 * (every cell renders neutral).
 */
export function phaseCellIntensity(
  stats: TaskStatsResponse,
  phase: Phase,
  status: PhaseStatus,
): number {
  const value = phaseCellCount(stats, phase, status);
  if (value <= 0) return 0;
  let maxCell = 0;
  for (const p of PHASE_DISPLAY_ORDER) {
    const inner = stats.phases.by_phase_status[p];
    for (const st of PHASE_STATUS_DISPLAY_ORDER) {
      const v = inner[st] ?? 0;
      if (v > maxCell) maxCell = v;
    }
  }
  if (maxCell <= 0) return 0;
  return value / maxCell;
}
