import type { TaskChangeFrame } from "../task-query/sseInvalidate";

export type SyncSchedule = "debounce" | "immediate" | "resync";

export type PendingDelta = {
  addTaskId?: string;
  markTaskEnriched?: string;
  addCycle?: { taskId: string; cycleId: string };
  markCycleEnriched?: { taskId: string; cycleId: string };
  clearAllPending?: boolean;
};

export type AgentRunProgressPayload = {
  taskId: string;
  cycleId: string;
  phaseSeq: number;
  progress: {
    kind: string;
    subtype?: string;
    message?: string;
    tool?: string;
  };
};

export type SyncEffect =
  | { kind: "invalidate"; queryKey: readonly unknown[] }
  | { kind: "patch_task_detail"; taskId: string; data: unknown }
  | { kind: "patch_cycle_detail"; taskId: string; cycleId: string; data: unknown }
  | { kind: "push_agent_run_progress"; payload: AgentRunProgressPayload }
  | { kind: "queue_progress_stream"; taskId: string; cycleId: string }
  | { kind: "rum_sse_resync" };

export type SyncFrameDecision = {
  schedule: SyncSchedule;
  pendingDelta: PendingDelta;
  effects: SyncEffect[];
};

export type SyncFlushDecision = {
  invalidateKeys: readonly (readonly unknown[])[];
};

export type DecideSyncFrameInput = {
  frame: TaskChangeFrame | null;
  shouldSuppressTaskEcho: (taskId: string) => boolean;
};
