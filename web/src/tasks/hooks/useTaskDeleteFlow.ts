import { useMutation, useQueryClient } from "@tanstack/react-query";
import { useCallback, useState } from "react";
import { deleteTask } from "../../api";
import { errorMessage } from "@/lib/errorMessage";
import { taskQueryKeys } from "../task-query";

/** Subset of `Task` the confirm dialog needs; widened so callers can pass plain rows. */
export type DeleteTargetInput = {
  id: string;
  title: string;
  parent_id?: string | null;
  /**
   * Optional total descendant count (children + grandchildren …) so the
   * confirm dialog can warn the user that DELETE cascades. Server-side
   * `DELETE /tasks/{id}` always cascades regardless of this hint
   * (docs/API-HTTP.md "DELETE /tasks/{id}"); the field is presentation-only
   * and defaults to 0 when omitted (callers without a tree in hand stay
   * source-compatible, the dialog simply omits the cascade warning line).
   */
  subtaskCount?: number;
};

export type DeleteTarget = {
  id: string;
  title: string;
  parent_id?: string;
  subtaskCount: number;
};

export type DeleteVariables = { id: string; parent_id?: string };

export type UseTaskDeleteFlowResult = {
  /** Currently-confirming target, or null when the dialog is closed. */
  deleteTarget: DeleteTarget | null;
  /** Open the confirmation dialog for `t`. Trims the `parent_id`. */
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
   * Clear the most recent settled state (error or success) without firing a
   * new request. Lets `useTasksApp` wipe a stale `deleteError` when the
   * confirm dialog closes (so a failed-then-cancelled delete doesn't render
   * its old error callout the next time the user reopens any delete dialog —
   * mirrors the `createMutation.reset()` lifecycle wired in session #33).
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
export function useTaskDeleteFlow(opts: {
  onDeleted?: (id: string) => void;
} = {}): UseTaskDeleteFlowResult {
  const queryClient = useQueryClient();
  const { onDeleted } = opts;
  const [deleteTarget, setDeleteTarget] = useState<DeleteTarget | null>(null);

  const mutation = useMutation({
    mutationFn: (input: DeleteVariables) => deleteTask(input.id),
    onSuccess: async (_, variables) => {
      const deletedId = variables.id;
      setDeleteTarget((prev) => (prev?.id === deletedId ? null : prev));
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.listRoot(),
      });
      await queryClient.invalidateQueries({ queryKey: ["task-stats"] });
      onDeleted?.(deletedId);
    },
  });

  const requestDelete = useCallback((t: DeleteTargetInput) => {
    const pid = t.parent_id?.trim();
    const count = Math.max(0, Math.trunc(t.subtaskCount ?? 0));
    setDeleteTarget({
      id: t.id,
      title: t.title,
      subtaskCount: count,
      ...(pid ? { parent_id: pid } : {}),
    });
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  const confirmDelete = useCallback(() => {
    if (!deleteTarget) return;
    mutation.mutate({
      id: deleteTarget.id,
      ...(deleteTarget.parent_id
        ? { parent_id: deleteTarget.parent_id }
        : {}),
    });
  }, [deleteTarget, mutation]);

  const resetError = useCallback(() => {
    if (mutation.isIdle) return;
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
