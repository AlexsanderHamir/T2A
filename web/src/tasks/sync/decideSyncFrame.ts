import { projectQueryKeys } from "@/projects/queryKeys";
import { settingsQueryKeys } from "@/settings/settingsQueryKeys";
import { taskQueryKeys } from "../task-query";
import type { DecideSyncFrameInput, SyncFrameDecision } from "./syncTypes";

export function decideSyncFrame(input: DecideSyncFrameInput): SyncFrameDecision {
  const { frame, shouldSuppressTaskEcho } = input;
  if (frame === null) {
    return { schedule: "debounce", pendingDelta: {}, effects: [] };
  }
  if (frame.kind === "task") {
    if (shouldSuppressTaskEcho(frame.taskId)) {
      return { schedule: "immediate", pendingDelta: {}, effects: [] };
    }
    const pendingDelta: SyncFrameDecision["pendingDelta"] = {
      addTaskId: frame.taskId,
    };
    const effects: SyncFrameDecision["effects"] = [];
    if (frame.data !== undefined) {
      effects.push({
        kind: "patch_task_detail",
        taskId: frame.taskId,
        data: frame.data,
      });
    }
    return { schedule: "debounce", pendingDelta, effects };
  }
  if (frame.kind === "project") {
    return {
      schedule: "immediate",
      pendingDelta: {},
      effects: [
        { kind: "invalidate", queryKey: projectQueryKeys.all },
        { kind: "invalidate", queryKey: taskQueryKeys.listRoot() },
      ],
    };
  }
  if (frame.kind === "project_context") {
    return {
      schedule: "immediate",
      pendingDelta: {},
      effects: [
        { kind: "invalidate", queryKey: projectQueryKeys.context(frame.projectId) },
        { kind: "invalidate", queryKey: projectQueryKeys.detail(frame.projectId) },
      ],
    };
  }
  if (frame.kind === "cycle") {
    const pendingDelta: SyncFrameDecision["pendingDelta"] = {
      addTaskId: frame.taskId,
      addCycle: { taskId: frame.taskId, cycleId: frame.cycleId },
    };
    const effects: SyncFrameDecision["effects"] = [];
    if (frame.data !== undefined) {
      effects.push({
        kind: "patch_cycle_detail",
        taskId: frame.taskId,
        cycleId: frame.cycleId,
        data: frame.data,
      });
    }
    return { schedule: "debounce", pendingDelta, effects };
  }
  if (frame.kind === "resync") {
    return {
      schedule: "resync",
      pendingDelta: { clearAllPending: true },
      effects: [
        { kind: "rum_sse_resync" },
        { kind: "invalidate", queryKey: taskQueryKeys.all },
        { kind: "invalidate", queryKey: taskQueryKeys.stats() },
        { kind: "invalidate", queryKey: taskQueryKeys.cycleFailuresRoot() },
        { kind: "invalidate", queryKey: settingsQueryKeys.app() },
      ],
    };
  }
  if (frame.kind === "progress") {
    return {
      schedule: "immediate",
      pendingDelta: {},
      effects: [
        {
          kind: "push_agent_run_progress",
          payload: {
            taskId: frame.taskId,
            cycleId: frame.cycleId,
            phaseSeq: frame.phaseSeq,
            progress: frame.progress,
          },
        },
        {
          kind: "queue_progress_stream",
          taskId: frame.taskId,
          cycleId: frame.cycleId,
        },
      ],
    };
  }
  if (frame.kind === "settings" || frame.kind === "agent_run_cancelled") {
    return {
      schedule: "immediate",
      pendingDelta: {},
      effects: [{ kind: "invalidate", queryKey: settingsQueryKeys.app() }],
    };
  }
  return { schedule: "debounce", pendingDelta: {}, effects: [] };
}
