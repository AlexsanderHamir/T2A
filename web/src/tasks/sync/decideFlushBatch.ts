import { taskQueryKeys } from "../task-query";
import {
  cycleEnrichmentKey,
  type PendingInvalidations,
} from "./syncConstants";
import type { SyncFlushDecision } from "./syncTypes";

export function decideFlushBatch(pending: PendingInvalidations): SyncFlushDecision {
  const taskIds = [...pending.tasks];
  const enrichedTaskIds = new Set(pending.enrichedTasks);
  const cycleEntries = [...pending.cycles.entries()];
  const enrichedCycles = new Set(pending.enrichedCycles);
  const keys: (readonly unknown[])[] = [];

  if (taskIds.length === 0 && cycleEntries.length === 0) {
    keys.push(
      taskQueryKeys.all,
      taskQueryKeys.stats(),
      taskQueryKeys.cycleFailuresRoot(),
    );
    return { invalidateKeys: keys };
  }

  if (taskIds.length > 0) {
    keys.push(taskQueryKeys.listRoot());
    const allTasksEnriched = taskIds.every((id) => enrichedTaskIds.has(id));
    if (!allTasksEnriched) {
      keys.push([...taskQueryKeys.all, "detail"]);
    }
  }

  for (const [taskId, cycleSet] of cycleEntries) {
    if (taskIds.includes(taskId)) {
      continue;
    }
    const allCyclesEnriched = [...cycleSet].every((cycleId) =>
      enrichedCycles.has(cycleEnrichmentKey(taskId, cycleId)),
    );
    if (!allCyclesEnriched) {
      keys.push(taskQueryKeys.cycles(taskId));
    }
  }

  const commitsTaskIds = new Set(taskIds);
  for (const [taskId] of cycleEntries) {
    commitsTaskIds.add(taskId);
  }
  for (const taskId of commitsTaskIds) {
    keys.push(taskQueryKeys.commits(taskId));
  }

  keys.push(taskQueryKeys.stats(), taskQueryKeys.cycleFailuresRoot());

  return { invalidateKeys: keys };
}
