/**
 * Public import surface for runtime status, RUM, and cycle display helpers.
 */
export { SystemStatusChip } from "./SystemStatusChip";

// Cycle/phase display helpers — re-exported so feature areas
// (e.g. tasks/components/task-detail/cycles) consume the same
// labels and pill classes.
export {
  cycleStatusLabel,
  phaseLabel,
  phaseStatusLabel,
  cycleStatusFillClass,
  phaseStatusFillClass,
  runnerLabel,
  formatRunnerModel,
  cycleRunnerChipClass,
} from "./cyclesViewModel";

// Operator-friendly duration formatter (matches journalctl/kubectl
// style: "12.3 s" / "12 min" / "12 h"). Used by task-detail cycles
// so the running-phase ticker reads identically to the system pane.
export { formatDurationSeconds } from "./systemHealthViewModel";

// RUM (Real-User-Monitoring) beacon. Use these helpers from feature
// hooks (mutations, SSE handlers) to feed the SLOs documented in
// docs/SLOs.md; the server-side counters in
// pkgs/tasks/middleware/metrics_http.go consume what we ship.
export {
  installRUM,
  mutationStarted as rumMutationStarted,
  mutationOptimisticApplied as rumMutationOptimisticApplied,
  mutationSettled as rumMutationSettled,
  mutationRolledBack as rumMutationRolledBack,
  sseReconnected as rumSSEReconnected,
  sseResyncReceived as rumSSEResyncReceived,
  type RUMMutationKind,
} from "./rum";
