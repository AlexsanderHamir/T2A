import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { patchTask as patchTaskApi } from "../../api";
import { errorMessage } from "@/lib/errorMessage";
import { taskQueryKeys } from "../task-query";
import {
  rumMutationRolledBack,
  rumMutationSettled,
} from "@/observability";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import type { Priority, Status, Task, TaskListResponse } from "@/types";
import {
  beginGuardedTaskWrite,
  cancelQueriesForKeys,
  endGuardedTaskWrite,
  mergePatchIntoTask,
  patchTaskInList,
  recordOptimisticApplied,
} from "@/tasks/mutations";

export type TaskPatchInput = {
  id: string;
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
  project_id?: string | null;
  project_context_item_ids?: string[];
  tags?: string[];
  milestone?: string | null;
  /** Per-task `cursor-agent --model` override; empty string clears override. */
  cursor_model: string;
  /** When set, included in PATCH body; omit when schedule is not editable. */
  pickup_not_before?: string | null;
};

export type UseTaskPatchFlowResult = {
  /** Fire the underlying PATCH /tasks/{id}; surface a banner via `patchError`. */
  patchTask: (input: TaskPatchInput) => void;
  patchPending: boolean;
  /** User-presentable error from the most recent patch attempt, or null. */
  patchError: string | null;
  /**
   * Clear the most recent settled state (error or success) without firing a
   * new request. Lets `useTasksApp` wipe a stale `patchError` when the edit
   * form closes (so a failed-then-cancelled save doesn't render its old
   * error callout the next time the user opens any edit dialog — mirrors
   * the `createMutation.reset()` lifecycle wired in session #33 and the
   * `useTaskDeleteFlow.resetError` companion shipped in session #34).
   */
  resetError: () => void;
};

interface PatchSnapshot {
  detail: Task | undefined;
  /** Prior list query data keyed by the React Query key the snapshot
   * was captured under. We restore each key on rollback so the cache
   * comes back identically even if the user navigated pages. */
  lists: Array<{ key: readonly unknown[]; data: TaskListResponse }>;
  /** Click moment for RUM latency observations. */
  startedAtMs: number;
  guarded: boolean;
}

/**
 * Owns the "save edits to a task" mutation. Now applies optimistically:
 * onMutate snapshots the detail + list cache, writes the merged patch
 * into both, bumps the optimistic-version counter so concurrent SSE
 * echoes can be suppressed, and records a RUM `mutation_started` +
 * `mutation_optimistic_applied` event. onError restores the snapshots
 * and surfaces a toast; onSettled invalidates so server truth
 * re-converges and decrements the version counter.
 *
 * Cross-cutting concerns are wired through a single `onPatched(id)`
 * callback so the parent can clear its edit form *only when the
 * resolving patch matches the currently-edited task*.
 */
export function useTaskPatchFlow(opts: {
  onPatched?: (id: string) => void;
} = {}): UseTaskPatchFlowResult {
  const queryClient = useQueryClient();
  const toast = useOptionalToast();
  const { optimisticMutationsEnabled } = useRolloutFlags();
  const { onPatched } = opts;

  const mutation = useMutation<unknown, unknown, TaskPatchInput, PatchSnapshot>({
    mutationFn: (input) =>
      patchTaskApi(input.id, {
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        project_id: input.project_id,
        project_context_item_ids: input.project_context_item_ids,
        tags: input.tags,
        milestone: input.milestone,
        cursor_model: input.cursor_model,
        ...(input.pickup_not_before !== undefined
          ? { pickup_not_before: input.pickup_not_before }
          : {}),
      }),
    onMutate: async (input) => {
      const guard = beginGuardedTaskWrite({
        taskId: input.id,
        optimisticEnabled: optimisticMutationsEnabled,
        rumKind: "task_patch",
      });
      if (!guard.guarded) {
        return { detail: undefined, lists: [], startedAtMs: guard.startedAtMs, guarded: false };
      }

      await cancelQueriesForKeys(queryClient, [
        taskQueryKeys.detail(input.id),
        taskQueryKeys.listRoot(),
      ]);

      const detailKey = taskQueryKeys.detail(input.id);
      const detailPrev = queryClient.getQueryData<Task>(detailKey);
      const { id: taskId, ...patchFields } = input;
      if (detailPrev) {
        queryClient.setQueryData<Task>(
          detailKey,
          mergePatchIntoTask(detailPrev, patchFields),
        );
      }

      const listSnapshots: PatchSnapshot["lists"] = [];
      const listEntries = queryClient.getQueriesData<TaskListResponse>({
        queryKey: taskQueryKeys.listRoot(),
      });
      for (const [key, data] of listEntries) {
        if (!data) continue;
        listSnapshots.push({ key, data });
        const next = patchTaskInList(data, taskId, patchFields);
        if (next) {
          queryClient.setQueryData<TaskListResponse>(key, next);
        }
      }

      recordOptimisticApplied("task_patch", guard.startedAtMs);

      return {
        detail: detailPrev,
        lists: listSnapshots,
        startedAtMs: guard.startedAtMs,
        guarded: true,
      };
    },
    onError: (err, input, context) => {
      const rolledBackSomething =
        !!context && (!!context.detail || context.lists.length > 0);
      if (context) {
        if (context.detail) {
          queryClient.setQueryData(taskQueryKeys.detail(input.id), context.detail);
        }
        for (const snap of context.lists) {
          queryClient.setQueryData(snap.key, snap.data);
        }
        // rolled_back is the numerator for slo_optimistic_rollback_rate;
        // only increment it when we ACTUALLY rolled back client state.
        // The pessimistic path returns an empty snapshot so nothing
        // was ever applied and reporting a rollback here would
        // inflate the SLI.
        if (rolledBackSomething) {
          rumMutationRolledBack(
            "task_patch",
            performance.now() - context.startedAtMs,
          );
        }
      }
      // The user-facing copy stays "reverted" in both paths: even in
      // the pessimistic branch nothing happened and "reverted" reads
      // as "no change was saved," which is accurate.
      toast.error("Couldn't save - reverted.");
      // Status code surfacing is best-effort; the patch flow funnels
      // every non-network error through errorMessage(), so we treat
      // it as 0 ("network/unknown") for the RUM bucket.
      rumMutationSettled(
        "task_patch",
        context ? performance.now() - context.startedAtMs : 0,
        0,
      );
      void err;
    },
    onSuccess: async (_, variables, context) => {
      const patchedId = variables.id;
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      onPatched?.(patchedId);
      if (context) {
        rumMutationSettled(
          "task_patch",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: (_data, _err, variables, context) => {
      if (context?.guarded) {
        endGuardedTaskWrite(variables.id);
      }
    },
  });

  const patchTask = useCallback(
    (input: TaskPatchInput) => {
      mutation.mutate(input);
    },
    [mutation],
  );

  const resetError = useCallback(() => {
    if (mutation.isIdle) return;
    mutation.reset();
  }, [mutation]);

  return {
    patchTask,
    patchPending: mutation.isPending,
    patchError: mutation.isError ? errorMessage(mutation.error) : null,
    resetError,
  };
}
