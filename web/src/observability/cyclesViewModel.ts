/**
 * Display helpers for cycle and phase status labels shared by task detail.
 */

import type {
  CycleMeta,
  CycleStatus,
  Phase,
  PhaseStatus,
} from "@/types/cycle";

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
      // Aborted intentionally maps to a dedicated class instead of
      // aliasing onto status-blocked; a deliberate stop should not read
      // like a blocked task in per-task cycle history.
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

/**
 * Operator-friendly label for a runner adapter name. Single source of
 * truth for the per-task UI (TaskDetailHeader, TaskCyclesPanel) and
 * related runtime chips.
 *
 * Unknown names fall through to their verbatim adapter identifier so
 * a new runner added by Phase 3 of the plan is labelled correctly
 * even before we update this table.
 */
const RUNNER_LABELS: Record<string, string> = {
  cursor: "Cursor CLI",
  "cursor-cli": "Cursor CLI",
  fake: "Fake runner",
};

export function runnerLabel(runnerName: string): string {
  const key = runnerName.trim();
  if (!key) return "unknown runner";
  return RUNNER_LABELS[key] ?? key;
}

/**
 * formatRunnerModel renders the combined "runner · model" chip copy
 * for the per-task UI (TaskDetailHeader, TaskCyclesPanel).
 *
 * Semantic contract matches docs/API-HTTP.md for cycle_meta:
 *  - empty runner  → "unknown runner"   (pre-feature cycles)
 *  - empty model   → "<runner> · default model"  (runner chose its own default)
 *  - both present  → "<runner> · <model>"
 *
 * Reads `cursor_model_effective` as the truth source: that's what
 * Prometheus and the breakdown panel also key on (plan decision D1).
 */
export function formatRunnerModel(meta: CycleMeta): string {
  const runner = runnerLabel(meta.runner);
  if (runner === "unknown runner") {
    return runner;
  }
  const model = meta.cursor_model_effective.trim();
  if (!model) {
    return `${runner} · default model`;
  }
  return `${runner} · ${model}`;
}

/**
 * CSS class for the runner/model chip rendered on CycleRow and the
 * TaskDetailHeader runtime pill. Returns the shared `cell-pill--runtime`
 * variant so the neutral-stone surface stays identical across views;
 * callers are expected to combine with `cell-pill` themselves.
 *
 * A dedicated variant (rather than reusing a status/priority class)
 * keeps the chip visually distinct from state-carrying pills so the
 * operator does not read runtime identity as a status signal.
 */
export function cycleRunnerChipClass(): string {
  return "cell-pill--runtime";
}
