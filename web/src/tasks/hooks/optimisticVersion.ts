/**
 * Module-scoped per-task "optimistic version" counter. Every time a
 * mutation hook applies an optimistic snapshot, it bumps the counter
 * for the task id; the SSE invalidation handler reads the counter and
 * **suppresses** its echo for any task whose version is still ahead
 * of the last seen invalidation tick.
 *
 * Why module-scope: the SSE handler and the mutation hooks live in
 * different React subtrees and can't share state via context without
 * forcing every mutation hook to subscribe. A module-level map is the
 * smallest dependency surface that gives the SSE handler "is there an
 * in-flight mutation on this task right now?" without re-architecting.
 *
 * Why a count instead of a boolean: rapid re-edits (user dragging a
 * priority pill across 3 values in 200ms) emit 3 onMutate calls; the
 * SSE echo for the FIRST one would otherwise un-arm the in-flight
 * flag and let the SECOND echo wipe the optimistic state. Counting
 * each onMutate / decrementing on each onSettled keeps the invariant
 * "version > seen → suppress" robust under bursts.
 */

const versions = new Map<string, number>();
const lastSeen = new Map<string, number>();

/** Increment the optimistic version for a task. Call from onMutate. */
export function bumpOptimisticVersion(taskId: string): number {
  const next = (versions.get(taskId) ?? 0) + 1;
  versions.set(taskId, next);
  return next;
}

/** Mark the SSE echo for a given task as observed (called from the
 * SSE invalidation path). Returns true if the caller should
 * SUPPRESS the echo (i.e. an in-flight mutation is still ahead). */
export function shouldSuppressSSEFor(taskId: string): boolean {
  const v = versions.get(taskId) ?? 0;
  const seen = lastSeen.get(taskId) ?? 0;
  if (v > seen) {
    lastSeen.set(taskId, v);
    return true;
  }
  return false;
}

/** Drop the bookkeeping for a task once the mutation settled. Call
 * from onSettled. The decrement is symmetric with bumpOptimisticVersion
 * so the counter doesn't grow unbounded. */
export function clearOptimisticVersion(taskId: string): void {
  const v = versions.get(taskId) ?? 0;
  if (v <= 1) {
    versions.delete(taskId);
    lastSeen.delete(taskId);
    return;
  }
  versions.set(taskId, v - 1);
}

/** Test-only: reset module state between cases. */
export function __resetOptimisticVersionsForTests(): void {
  versions.clear();
  lastSeen.clear();
}
