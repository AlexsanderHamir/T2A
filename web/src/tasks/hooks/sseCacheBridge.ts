import type { QueryClient } from "@tanstack/react-query";
import { parseTask, parseTaskCycleDetail } from "@/api/parseTaskApi";
import { projectQueryKeys } from "@/projects/queryKeys";
import { rumSSEResyncReceived } from "@/observability";
import { parseTaskChangeFrame, settingsQueryKeys, taskQueryKeys } from "../task-query";
import { pushAgentRunProgress } from "./useAgentRunProgress";
import { shouldSuppressSSEFor } from "./optimisticVersion";
import {
  clearPending,
  cycleEnrichmentKey,
  type PendingInvalidations,
} from "./sseInvalidationScheduler";

export type FrameDispatchResult =
  | { kind: "debounce" }
  | { kind: "immediate" }
  | { kind: "resync" };

export function flushStreamInvalidation(
  queryClient: QueryClient,
  pending: PendingInvalidations,
): void {
  const taskIds = [...pending.tasks];
  const enrichedTaskIds = new Set(pending.enrichedTasks);
  const cycleEntries = [...pending.cycles.entries()];
  const enrichedCycles = new Set(pending.enrichedCycles);
  clearPending(pending);
  if (taskIds.length === 0 && cycleEntries.length === 0) {
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
    void queryClient.invalidateQueries({
      queryKey: taskQueryKeys.cycleFailuresRoot(),
    });
    return;
  }
  if (taskIds.length > 0) {
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
    const allTasksEnriched = taskIds.every((id) => enrichedTaskIds.has(id));
    if (!allTasksEnriched) {
      void queryClient.invalidateQueries({
        queryKey: [...taskQueryKeys.all, "detail"],
      });
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
      void queryClient.invalidateQueries({
        queryKey: taskQueryKeys.cycles(taskId),
      });
    }
  }
  const commitsTaskIds = new Set(taskIds);
  for (const [taskId] of cycleEntries) {
    commitsTaskIds.add(taskId);
  }
  for (const taskId of commitsTaskIds) {
    void queryClient.invalidateQueries({
      queryKey: taskQueryKeys.commits(taskId),
    });
  }
  void queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
  void queryClient.invalidateQueries({
    queryKey: taskQueryKeys.cycleFailuresRoot(),
  });
}

export function flushProgressStreamInvalidation(
  queryClient: QueryClient,
  pendingProgressStreams: Map<string, { taskId: string; cycleId: string }>,
): void {
  const streams = [...pendingProgressStreams.values()];
  pendingProgressStreams.clear();
  for (const stream of streams) {
    void queryClient.invalidateQueries({
      queryKey: taskQueryKeys.cycleStream(stream.taskId, stream.cycleId),
    });
  }
}

export function dispatchTaskChangeFrame(
  data: string,
  queryClient: QueryClient,
  pending: PendingInvalidations,
  onProgressStream: (taskId: string, cycleId: string) => void,
): FrameDispatchResult {
  const frame = parseTaskChangeFrame(data);
  if (frame === null) {
    return { kind: "debounce" };
  }
  if (frame.kind === "task") {
    if (shouldSuppressSSEFor(frame.taskId)) {
      return { kind: "immediate" };
    }
    pending.tasks.add(frame.taskId);
    if (frame.data !== undefined) {
      try {
        const parsed = parseTask(frame.data);
        queryClient.setQueryData(taskQueryKeys.detail(frame.taskId), parsed);
        pending.enrichedTasks.add(frame.taskId);
      } catch {
        /* fall back to invalidate-and-refetch */
      }
    }
    return { kind: "debounce" };
  }
  if (frame.kind === "project") {
    void queryClient.invalidateQueries({ queryKey: projectQueryKeys.all });
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
    return { kind: "immediate" };
  }
  if (frame.kind === "project_context") {
    void queryClient.invalidateQueries({ queryKey: projectQueryKeys.context(frame.projectId) });
    void queryClient.invalidateQueries({ queryKey: projectQueryKeys.detail(frame.projectId) });
    return { kind: "immediate" };
  }
  if (frame.kind === "cycle") {
    pending.tasks.add(frame.taskId);
    let bucket = pending.cycles.get(frame.taskId);
    if (bucket === undefined) {
      bucket = new Set();
      pending.cycles.set(frame.taskId, bucket);
    }
    bucket.add(frame.cycleId);
    if (frame.data !== undefined) {
      try {
        const parsedCycle = parseTaskCycleDetail(frame.data);
        queryClient.setQueryData(
          taskQueryKeys.cycle(frame.taskId, frame.cycleId),
          parsedCycle,
        );
        pending.enrichedCycles.add(cycleEnrichmentKey(frame.taskId, frame.cycleId));
      } catch {
        /* fall back to invalidate-and-refetch */
      }
    }
    return { kind: "debounce" };
  }
  if (frame.kind === "resync") {
    rumSSEResyncReceived();
    clearPending(pending);
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
    void queryClient.invalidateQueries({
      queryKey: taskQueryKeys.cycleFailuresRoot(),
    });
    void queryClient.invalidateQueries({
      queryKey: settingsQueryKeys.app(),
    });
    return { kind: "resync" };
  }
  if (frame.kind === "progress") {
    pushAgentRunProgress({
      taskId: frame.taskId,
      cycleId: frame.cycleId,
      phaseSeq: frame.phaseSeq,
      progress: frame.progress,
    });
    onProgressStream(frame.taskId, frame.cycleId);
    return { kind: "immediate" };
  }
  if (frame.kind === "settings" || frame.kind === "agent_run_cancelled") {
    void queryClient.invalidateQueries({
      queryKey: settingsQueryKeys.app(),
    });
    return { kind: "immediate" };
  }
  return { kind: "debounce" };
}
