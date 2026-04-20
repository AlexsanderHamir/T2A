import type { Priority, Status, TaskStatsResponse } from "@/types/task";

/**
 * Display order for the status stacked bar. Mirrors the workflow direction
 * (intake → in flight → terminal), not alphabetical, so the colored bar
 * reads left-to-right as a pipeline from "ready to start" to "done/failed".
 */
export const STATUS_DISPLAY_ORDER: Status[] = [
  "ready",
  "running",
  "blocked",
  "review",
  "done",
  "failed",
];

/** Display order: ascending escalation (low → critical) for the priority bar. */
export const PRIORITY_DISPLAY_ORDER: Priority[] = [
  "low",
  "medium",
  "high",
  "critical",
];

const STATUS_LABELS: Record<Status, string> = {
  ready: "Ready",
  running: "Running",
  blocked: "Blocked",
  review: "Review",
  done: "Done",
  failed: "Failed",
};

const PRIORITY_LABELS: Record<Priority, string> = {
  low: "Low",
  medium: "Medium",
  high: "High",
  critical: "Critical",
};

export function statusLabel(s: Status): string {
  return STATUS_LABELS[s];
}

export function priorityLabel(p: Priority): string {
  return PRIORITY_LABELS[p];
}

/** CSS class for a status segment / swatch. Matches the task list pills. */
export function statusFillClass(s: Status): string {
  return `cell-pill--status-${s}`;
}

/** CSS class for a priority segment / swatch. Matches the task list pills. */
export function priorityFillClass(p: Priority): string {
  return `cell-pill--priority-${p}`;
}

/**
 * Sum of `running + blocked + review` from the by_status map. The wire
 * contract guarantees absent buckets are missing entries (never null), so
 * the `?? 0` short-circuit is the documented zero-coalesce.
 */
export function inFlightCount(stats: TaskStatsResponse): number {
  const s = stats.by_status;
  return (s.running ?? 0) + (s.blocked ?? 0) + (s.review ?? 0);
}

export function doneCount(stats: TaskStatsResponse): number {
  return stats.by_status.done ?? 0;
}

export function failedCount(stats: TaskStatsResponse): number {
  return stats.by_status.failed ?? 0;
}
