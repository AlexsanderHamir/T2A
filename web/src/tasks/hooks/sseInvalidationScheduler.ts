/**
 * Debounce and max-wait tuning for SSE-driven React Query invalidation.
 * See useTaskEventStream for workload rationale.
 */
export const SSE_INVALIDATE_WINDOW_MS = 900;
export const SSE_INVALIDATE_MAX_WAIT_MS = 2500;
export const PROGRESS_STREAM_INVALIDATE_WINDOW_MS = 5000;
export const PROGRESS_STREAM_INVALIDATE_MAX_WAIT_MS = 10000;

export type PendingInvalidations = {
  tasks: Set<string>;
  enrichedTasks: Set<string>;
  cycles: Map<string, Set<string>>;
  enrichedCycles: Set<string>;
};

export type PendingProgressStreams = Map<string, { taskId: string; cycleId: string }>;

export function emptyPending(): PendingInvalidations {
  return {
    tasks: new Set(),
    enrichedTasks: new Set(),
    cycles: new Map(),
    enrichedCycles: new Set(),
  };
}

export function clearPending(p: PendingInvalidations): void {
  p.tasks.clear();
  p.enrichedTasks.clear();
  p.cycles.clear();
  p.enrichedCycles.clear();
}

export function cycleEnrichmentKey(taskId: string, cycleId: string): string {
  return `${taskId}\u0000${cycleId}`;
}

/** Delay for trailing debounce respecting max-wait since firstQueuedAt. */
export function debounceDelayMs(
  now: number,
  firstQueuedAt: number | null,
  windowMs: number,
  maxWaitMs: number,
): number {
  const base = firstQueuedAt ?? now;
  const elapsedSinceFirst = now - base;
  const remainingBudget = maxWaitMs - elapsedSinceFirst;
  return Math.max(0, Math.min(windowMs, remainingBudget));
}
