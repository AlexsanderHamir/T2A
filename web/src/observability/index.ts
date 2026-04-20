/**
 * Public import surface for the observability feature
 * (.cursor/rules/CODE_STANDARDS.mdc). The app shell imports `@/observability`
 * instead of deep paths.
 */
export { ObservabilityPage } from "./ObservabilityPage";
export { SystemStatusChip } from "./SystemStatusChip";

// Cycle/phase display helpers — re-exported so feature areas
// (e.g. tasks/components/task-detail/cycles) consume the same
// labels and pill classes as the observability page itself.
// Single source of truth for label strings, display order, and
// CSS-class mapping; otherwise a status rename in one place would
// silently desync the two views.
export {
  cycleStatusLabel,
  phaseLabel,
  phaseStatusLabel,
  cycleStatusFillClass,
  phaseStatusFillClass,
  PHASE_DISPLAY_ORDER,
  RUNNER_LABELS,
  runnerLabel,
  formatRunnerModel,
  cycleRunnerChipClass,
} from "./cyclesViewModel";

// Operator-friendly duration formatter (matches journalctl/kubectl
// style: "12.3 s" / "12 min" / "12 h"). Used by task-detail cycles
// so the running-phase ticker reads identically to the system pane.
export { formatDurationSeconds } from "./systemHealthViewModel";
