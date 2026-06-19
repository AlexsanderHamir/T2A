import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useState } from "react";
import { deleteTask } from "../../api";
import { errorMessage } from "@/lib/errorMessage";
import {
  rumMutationOptimisticApplied,
  rumMutationRolledBack,
  rumMutationSettled,
  rumMutationStarted,
} from "@/observability";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import { taskQueryKeys } from "../task-query";
import { beginTaskMutation, endTaskMutation } from "@/tasks/sync";
import type { Task, TaskListResponse } from "@/types";

/** Subset of `Task` the confirm dialog needs; widened so callers can pass plain rows. */
export type DeleteTargetInput = {
  id: string;
  title: string;
};

export type DeleteTarget = {
  id: string;
  title: string;
};

export type DeleteVariables = { id: string };

export type UseTaskDeleteFlowResult = {
  /** Currently-confirming target, or null when the dialog is closed. */
  deleteTarget: DeleteTarget | null;
  /** Open the confirmation dialog for `t`. */
  requestDelete: (t: DeleteTargetInput) => void;
  /** Close the confirmation dialog without deleting. */
  cancelDelete: () => void;
  /** Fire the delete for the current `deleteTarget`; no-op if none is set. */
  confirmDelete: () => void;
  deletePending: boolean;
  /** User-presentable error message for the most recent delete attempt, or null. */
  deleteError: string | null;
  /** True from the moment the delete settles successfully until `requestDelete` is called again. */
  deleteSuccess: boolean;
  /** The variables of the most recent settled delete (used by the detail page to navigate away). */
  deleteVariables: DeleteVariables | undefined;
  /**
   * Clear the most recent error without firing a new request. Successful
   * delete variables must remain visible for detail-page navigation after
   * the confirm dialog closes.
   */
  resetError: () => void;
};

/**
 * Owns the in-app delete-confirmation flow that used to live inline in
 * `useTasksApp`. We avoid `window.confirm` because it breaks input focus in
 * some browsers (see comment on the original `deleteTarget` state).
 *
 * The hook does **not** know about `editing`, the routing, or the global
 * error banner. Cross-cutting concerns are wired through `onDeleted` so the
 * parent can react (e.g. clear the edit form for the just-deleted task)
 * without this hook depending on the rest of `useTasksApp`'s state.
 *
 * Query invalidation is handled here because the list + stats refresh is
 * intrinsic to "a delete succeeded".
 *
 * The internal `deleteTarget` clear on success is id-aware (mirrors the
 * `useTaskPatchFlow` race fix): if a delete settles *after* the user has
 * already opened the confirm dialog for a *different* row, we leave that
 * second confirm dialog up instead of silently dismissing it.
 */
interface DeleteSnapshot {
  detail: Task | undefined;
  /** Per-list-key snapshots so we can restore each cached page on rollback. */
  lists: Array<{ key: readonly unknown[]; data: TaskListResponse }>;
  startedAtMs: number;
}

/** Remove the task with id `removeId` from a cached TaskListResponse.
 * Returns null when the id wasn't found so callers can skip the cache write. */
function removeTaskFromList(
  list: TaskListResponse,
  removeId: string,
): TaskListResponse | null {
  const nextTasks = list.tasks.filter((t) => t.id !== removeId);
  if (nextTasks.length === list.tasks.length) return null;
  return { ...list, tasks: nextTasks };
}

export function useTaskDeleteFlow(opts: {
  onDeleted?: (id: string) => void;
} = {}): UseTaskDeleteFlowResult {
  const queryClient = useQueryClient();
  const toast = useOptionalToast();
  const { optimisticMutationsEnabled } = useRolloutFlags();
  const { onDeleted } = opts;
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);

  const mutation = useMutation<unknown, unknown, DeleteVariables, DeleteSnapshot>({
    mutationFn: (input) => deleteTask(input.id),
    onMutate: async (input) => {
      const startedAtMs = performance.now();
      rumMutationStarted("task_delete");
      // See useTaskPatchFlow: pessimistic path returns an empty
      // snapshot, so onError can no-op and we don't pollute the
      // rolled_back SLI. endTaskMutation stays safe when
      // nothing was bumped.
      if (!optimisticMutationsEnabled) {
        return { detail: undefined, lists: [], startedAtMs };
      }
      beginTaskMutation(input.id);

      await queryClient.cancelQueries({ queryKey: taskQueryKeys.listRoot() });
      await queryClient.cancelQueries({ queryKey: taskQueryKeys.detail(input.id) });

      const detailKey = taskQueryKeys.detail(input.id);
      const detailPrev = queryClient.getQueryData<Task>(detailKey);
      // Drop the detail entry so any open detail page sees the
      // "task not found / deleted" empty state immediately. The
      // server-truth refetch on rollback restores the full Task.
      queryClient.removeQueries({ queryKey: detailKey });

      const listSnapshots: DeleteSnapshot["lists"] = [];
      const listEntries = queryClient.getQueriesData<TaskListResponse>({
        queryKey: taskQueryKeys.listRoot(),
      });
      for (const [key, data] of listEntries) {
        if (!data) continue;
        listSnapshots.push({ key, data });
        const next = removeTaskFromList(data, input.id);
        if (next) {
          queryClient.setQueryData<TaskListResponse>(key, next);
        }
      }

      rumMutationOptimisticApplied("task_delete", performance.now() - startedAtMs);
      return { detail: detailPrev, lists: listSnapshots, startedAtMs };
    },
    onError: (_err, input, context) => {
      const rolledBackSomething =
        !!context && (!!context.detail || context.lists.length > 0);
      if (context) {
        if (context.detail) {
          queryClient.setQueryData(taskQueryKeys.detail(input.id), context.detail);
        }
        for (const snap of context.lists) {
          queryClient.setQueryData(snap.key, snap.data);
        }
        if (rolledBackSomething) {
          rumMutationRolledBack(
            "task_delete",
            performance.now() - context.startedAtMs,
          );
        }
      }
      toast.error("Couldn't delete - reverted.");
      rumMutationSettled(
        "task_delete",
        context ? performance.now() - context.startedAtMs : 0,
        0,
      );
    },
    onSuccess: async (_, variables, context) => {
      const deletedId = variables.id;
      setDeleteTarget((prev) => (prev?.id === deletedId ? null : prev));
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.listRoot(),
      });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
      onDeleted?.(deletedId);
      if (context) {
        rumMutationSettled(
          "task_delete",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: (_data, _err, variables) => {
      endTaskMutation(variables.id);
    },
  });

  const requestDelete = useCallback((t: DeleteTargetInput) => {
    setDeleteTarget({
      id: t.id,
      title: t.title,
    });
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  const confirmDelete = useCallback(() => {
    if (!deleteTarget) return;
    mutation.mutate({
      id: deleteTarget.id,
    });
  }, [deleteTarget, mutation]);

  const resetError = useCallback(() => {
    if (!mutation.isError) return;
    mutation.reset();
  }, [mutation]);

  return {
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
    deletePending: mutation.isPending,
    deleteError: mutation.isError ? errorMessage(mutation.error) : null,
    deleteSuccess: mutation.isSuccess,
    deleteVariables: mutation.variables,
    resetError,
  };
}
