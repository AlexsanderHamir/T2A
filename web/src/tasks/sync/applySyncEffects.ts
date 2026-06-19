import type { QueryClient } from "@tanstack/react-query";
import { parseTask, parseTaskCycleDetail } from "@/api/parseTaskApi";
import { rumSSEResyncReceived } from "@/observability";
import { bustQueryPersistCache } from "@/lib/queryPersist";
import { taskQueryKeys } from "../task-query";
import { pushAgentRunProgress } from "../hooks/useAgentRunProgress";
import type { PendingDelta, SyncEffect } from "./syncTypes";
import {
  cycleEnrichmentKey,
  type PendingInvalidations,
  type PendingProgressStreams,
} from "./syncConstants";

export function mergePendingDelta(
  pending: PendingInvalidations,
  delta: PendingDelta,
): void {
  if (delta.clearAllPending) {
    pending.tasks.clear();
    pending.enrichedTasks.clear();
    pending.cycles.clear();
    pending.enrichedCycles.clear();
  }
  if (delta.addTaskId !== undefined) {
    pending.tasks.add(delta.addTaskId);
  }
  if (delta.markTaskEnriched !== undefined) {
    pending.enrichedTasks.add(delta.markTaskEnriched);
  }
  if (delta.addCycle !== undefined) {
    const { taskId, cycleId } = delta.addCycle;
    let bucket = pending.cycles.get(taskId);
    if (bucket === undefined) {
      bucket = new Set();
      pending.cycles.set(taskId, bucket);
    }
    bucket.add(cycleId);
  }
  if (delta.markCycleEnriched !== undefined) {
    const { taskId, cycleId } = delta.markCycleEnriched;
    pending.enrichedCycles.add(cycleEnrichmentKey(taskId, cycleId));
  }
}

export function applySyncEffects(
  queryClient: QueryClient,
  effects: SyncEffect[],
): PendingDelta {
  const enrichmentMarks: PendingDelta = {};

  for (const effect of effects) {
    if (effect.kind === "invalidate") {
      void queryClient.invalidateQueries({ queryKey: effect.queryKey });
      continue;
    }
    if (effect.kind === "rum_sse_resync") {
      rumSSEResyncReceived();
      bustQueryPersistCache();
      continue;
    }
    if (effect.kind === "patch_task_detail") {
      try {
        const parsed = parseTask(effect.data);
        queryClient.setQueryData(taskQueryKeys.detail(effect.taskId), parsed);
        enrichmentMarks.markTaskEnriched = effect.taskId;
      } catch {
        /* fall back to invalidate-and-refetch on flush */
      }
      continue;
    }
    if (effect.kind === "patch_cycle_detail") {
      try {
        const parsedCycle = parseTaskCycleDetail(effect.data);
        queryClient.setQueryData(
          taskQueryKeys.cycle(effect.taskId, effect.cycleId),
          parsedCycle,
        );
        enrichmentMarks.markCycleEnriched = {
          taskId: effect.taskId,
          cycleId: effect.cycleId,
        };
      } catch {
        /* fall back to invalidate-and-refetch on flush */
      }
      continue;
    }
    if (effect.kind === "push_agent_run_progress") {
      pushAgentRunProgress(effect.payload);
    }
  }

  return enrichmentMarks;
}

export function applyFlushDecision(
  queryClient: QueryClient,
  invalidateKeys: readonly (readonly unknown[])[],
): void {
  for (const queryKey of invalidateKeys) {
    void queryClient.invalidateQueries({ queryKey });
  }
}

export function flushProgressStreams(
  queryClient: QueryClient,
  pendingProgressStreams: PendingProgressStreams,
): void {
  const streams = [...pendingProgressStreams.values()];
  pendingProgressStreams.clear();
  for (const stream of streams) {
    void queryClient.invalidateQueries({
      queryKey: taskQueryKeys.cycleStream(stream.taskId, stream.cycleId),
    });
  }
}
