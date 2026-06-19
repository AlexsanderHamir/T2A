import type { TaskCycle } from "@/types/cycle";

function retryModeFromMeta(cycle: TaskCycle): string {
  const raw = cycle.meta.retry_mode;
  return typeof raw === "string" ? raw.trim() : "";
}

/**
 * Human-readable lineage suffix for operator retry attempts, e.g.
 * "started over from attempt 2" or "resumed from attempt 2".
 */
export function formatCycleLineageLabel(
  cycle: TaskCycle,
  cyclesById: ReadonlyMap<string, TaskCycle>,
): string | null {
  const parentId = cycle.parent_cycle_id?.trim();
  if (!parentId) {
    return null;
  }
  const parent = cyclesById.get(parentId);
  const parentAttempt = parent?.attempt_seq;
  if (!parentAttempt) {
    return null;
  }
  switch (retryModeFromMeta(cycle)) {
    case "fresh":
      return `started over from attempt ${parentAttempt}`;
    case "resume":
      return `resumed from attempt ${parentAttempt}`;
    default:
      return `from attempt ${parentAttempt}`;
  }
}
