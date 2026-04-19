import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback } from "react";
import { patchTask as patchTaskApi } from "../../api";
import { errorMessage } from "@/lib/errorMessage";
import { taskQueryKeys } from "../task-query";
import type { Priority, Status, TaskType } from "@/types";

export type TaskPatchInput = {
  id: string;
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
  task_type: TaskType;
  checklist_inherit: boolean;
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

/**
 * Owns the "save edits to a task" mutation that used to live inline in
 * `useTasksApp`. Pulled out for the same reasons as `useTaskDeleteFlow`:
 * the mutation, its query invalidations (list + per-task detail + stats),
 * and the `onPatched` cross-cut all belong together as one cohesive slice.
 *
 * Cross-cutting concerns are wired through a single `onPatched(id)`
 * callback so the parent can clear its edit form *only when the resolving
 * patch matches the currently-edited task*. The previous inline version
 * cleared `editing` unconditionally, which would also drop a quickly-opened
 * second edit form if a stale first patch settled afterwards — the id
 * compare in the parent's `onPatched` handler closes that race.
 */
export function useTaskPatchFlow(opts: {
  onPatched?: (id: string) => void;
} = {}): UseTaskPatchFlowResult {
  const queryClient = useQueryClient();
  const { onPatched } = opts;

  const mutation = useMutation({
    mutationFn: (input: TaskPatchInput) =>
      patchTaskApi(input.id, {
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        task_type: input.task_type,
        checklist_inherit: input.checklist_inherit,
      }),
    onSuccess: async (_, variables) => {
      const patchedId = variables.id;
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
      onPatched?.(patchedId);
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
