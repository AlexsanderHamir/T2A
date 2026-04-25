import { useSyncExternalStore } from "react";

export type AgentRunProgress = {
  kind: string;
  subtype?: string;
  message?: string;
  tool?: string;
};

export type AgentRunProgressFrame = {
  taskId: string;
  cycleId: string;
  phaseSeq: number;
  progress: AgentRunProgress;
};

export type AgentRunProgressItem = AgentRunProgressFrame & {
  receivedAt: number;
};

const MAX_ITEMS_PER_PHASE = 5;
const MAX_TRACKED_PHASES = 50;
const EMPTY_PROGRESS: AgentRunProgressItem[] = [];

const progressByPhase = new Map<string, AgentRunProgressItem[]>();
const listeners = new Set<() => void>();

function keyFor(taskId: string, cycleId: string, phaseSeq: number): string {
  return `${taskId}:${cycleId}:${phaseSeq}`;
}

function emitChange(): void {
  for (const listener of listeners) {
    listener();
  }
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener);
  return () => {
    listeners.delete(listener);
  };
}

function snapshotFor(taskId: string, cycleId: string, phaseSeq: number): AgentRunProgressItem[] {
  return progressByPhase.get(keyFor(taskId, cycleId, phaseSeq)) ?? EMPTY_PROGRESS;
}

export function pushAgentRunProgress(frame: AgentRunProgressFrame): void {
  if (
    frame.taskId.trim() === "" ||
    frame.cycleId.trim() === "" ||
    frame.phaseSeq <= 0 ||
    frame.progress.kind.trim() === ""
  ) {
    return;
  }
  const key = keyFor(frame.taskId, frame.cycleId, frame.phaseSeq);
  const current = progressByPhase.get(key) ?? [];
  progressByPhase.set(key, [
    ...current,
    { ...frame, receivedAt: Date.now() },
  ].slice(-MAX_ITEMS_PER_PHASE));

  while (progressByPhase.size > MAX_TRACKED_PHASES) {
    const oldest = progressByPhase.keys().next().value as string | undefined;
    if (oldest === undefined) break;
    progressByPhase.delete(oldest);
  }
  emitChange();
}

export function useAgentRunProgress(
  taskId: string,
  cycleId: string,
  phaseSeq: number,
): AgentRunProgressItem[] {
  return useSyncExternalStore(
    subscribe,
    () => snapshotFor(taskId, cycleId, phaseSeq),
    () => EMPTY_PROGRESS,
  );
}
