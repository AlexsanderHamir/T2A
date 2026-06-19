import type { QueryClient } from "@tanstack/react-query";
import {
  rumMutationOptimisticApplied,
  rumMutationStarted,
  type RUMMutationKind,
} from "@/observability";
import { beginTaskMutation, endTaskMutation } from "@/tasks/sync";

export type GuardedWriteContext = {
  startedAtMs: number;
  guarded: boolean;
};

export function beginGuardedTaskWrite(input: {
  taskId: string;
  optimisticEnabled: boolean;
  rumKind: RUMMutationKind;
}): GuardedWriteContext {
  const startedAtMs = performance.now();
  rumMutationStarted(input.rumKind);
  if (!input.optimisticEnabled) {
    return { startedAtMs, guarded: false };
  }
  beginTaskMutation(input.taskId);
  return { startedAtMs, guarded: true };
}

export function endGuardedTaskWrite(taskId: string): void {
  endTaskMutation(taskId);
}

export async function cancelQueriesForKeys(
  queryClient: QueryClient,
  keys: readonly (readonly unknown[])[],
): Promise<void> {
  await Promise.all(
    keys.map((queryKey) => queryClient.cancelQueries({ queryKey })),
  );
}

export function recordOptimisticApplied(
  rumKind: RUMMutationKind,
  startedAtMs: number,
): void {
  rumMutationOptimisticApplied(rumKind, performance.now() - startedAtMs);
}
