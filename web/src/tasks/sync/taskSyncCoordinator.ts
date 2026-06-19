import type { QueryClient } from "@tanstack/react-query";
import { parseTaskChangeFrame } from "../task-query";
import {
  applyFlushDecision,
  applySyncEffects,
  flushProgressStreams,
  mergePendingDelta,
} from "./applySyncEffects";
import { decideFlushBatch } from "./decideFlushBatch";
import { decideSyncFrame } from "./decideSyncFrame";
import {
  clearPending,
  emptyPending,
  type PendingInvalidations,
  type PendingProgressStreams,
} from "./syncConstants";
import { shouldSuppressTaskMutationEcho } from "./mutationGuard";
import type { SyncSchedule } from "./syncTypes";

export type FrameDispatchResult =
  | { kind: "debounce" }
  | { kind: "immediate" }
  | { kind: "resync" };

function scheduleToDispatchResult(schedule: SyncSchedule): FrameDispatchResult {
  if (schedule === "resync") {
    return { kind: "resync" };
  }
  if (schedule === "immediate") {
    return { kind: "immediate" };
  }
  return { kind: "debounce" };
}

export type TaskSyncCoordinator = {
  pending: PendingInvalidations;
  pendingProgressStreams: PendingProgressStreams;
  handleRawFrame: (
    data: string,
    onProgressStream: (taskId: string, cycleId: string) => void,
  ) => FrameDispatchResult;
  flushStreamInvalidation: () => void;
  flushProgressStreamInvalidation: () => void;
  dispose: () => void;
};

export function createTaskSyncCoordinator(queryClient: QueryClient): TaskSyncCoordinator {
  const pending = emptyPending();
  const pendingProgressStreams: PendingProgressStreams = new Map();

  return {
    pending,
    pendingProgressStreams,
    handleRawFrame(data, onProgressStream) {
      const frame = parseTaskChangeFrame(data);
      const decision = decideSyncFrame({
        frame,
        shouldSuppressTaskEcho: shouldSuppressTaskMutationEcho,
      });

      const enrichmentMarks = applySyncEffects(queryClient, decision.effects);
      mergePendingDelta(pending, decision.pendingDelta);
      mergePendingDelta(pending, enrichmentMarks);

      for (const effect of decision.effects) {
        if (effect.kind === "queue_progress_stream") {
          onProgressStream(effect.taskId, effect.cycleId);
        }
      }

      return scheduleToDispatchResult(decision.schedule);
    },
    flushStreamInvalidation() {
      const flushDecision = decideFlushBatch(pending);
      clearPending(pending);
      applyFlushDecision(queryClient, flushDecision.invalidateKeys);
    },
    flushProgressStreamInvalidation() {
      flushProgressStreams(queryClient, pendingProgressStreams);
    },
    dispose() {
      clearPending(pending);
      pendingProgressStreams.clear();
    },
  };
}
