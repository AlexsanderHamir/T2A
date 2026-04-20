import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  deleteChecklistItem,
  patchChecklistItemText,
} from "@/api";
import {
  rumMutationOptimisticApplied,
  rumMutationRolledBack,
  rumMutationSettled,
  rumMutationStarted,
  type RUMMutationKind,
} from "@/observability";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import { taskQueryKeys } from "../task-query";
import type { TaskChecklistItemView, TaskChecklistResponse } from "@/types";

interface ChecklistOptimisticContext {
  prev: TaskChecklistResponse | undefined;
  startedAtMs: number;
  /** Identifier we used for the synthetic add — onSuccess swaps it
   * for the real id in the cache so subsequent edits don't reference
   * the temp id. */
  tempItemId?: string;
}

let optimisticChecklistTempCounter = 0;
function nextOptimisticChecklistId(): string {
  optimisticChecklistTempCounter += 1;
  return `optimistic-${optimisticChecklistTempCounter}`;
}

function snapshotChecklist(
  queryClient: QueryClient,
  taskId: string,
): TaskChecklistResponse | undefined {
  return queryClient.getQueryData<TaskChecklistResponse>(taskQueryKeys.checklist(taskId));
}

function restoreChecklist(
  queryClient: QueryClient,
  taskId: string,
  prev: TaskChecklistResponse | undefined,
): void {
  if (prev !== undefined) {
    queryClient.setQueryData(taskQueryKeys.checklist(taskId), prev);
  } else {
    queryClient.removeQueries({ queryKey: taskQueryKeys.checklist(taskId) });
  }
}

function recordRollback(
  kind: RUMMutationKind,
  startedAtMs: number,
): void {
  rumMutationRolledBack(kind, performance.now() - startedAtMs);
  rumMutationSettled(kind, performance.now() - startedAtMs, 0);
}

export function useTaskDetailChecklist(taskId: string) {
  const queryClient = useQueryClient();
  const toast = useOptionalToast();
  const { optimisticMutationsEnabled } = useRolloutFlags();
  const [checklistModalOpen, setChecklistModalOpen] = useState(false);
  const [newChecklistText, setNewChecklistText] = useState("");
  const [editCriterionModalOpen, setEditCriterionModalOpen] = useState(false);
  const [editingChecklistItemId, setEditingChecklistItemId] = useState<
    string | null
  >(null);
  const [editChecklistText, setEditChecklistText] = useState("");

  /**
   * Per-submission token for `addChecklistMutation`. The new criterion
   * has no caller-supplied identity (the text is a free-form string),
   * so we use a monotonically-increasing counter as the
   * "this is the add the user is currently waiting on" identity. Bumped
   * (a) on every `submitNewChecklistCriterion` call so each in-flight
   * mutation carries its own snapshot, AND (b) inside
   * `closeChecklistModal` / `openChecklistModal` / `openEditCriterionModal`
   * / the `taskId`-change effect so any user-initiated dismiss / reopen /
   * mode-switch supersedes any in-flight create. Same shape as
   * `submissionTokenRef` in `useTaskDetailSubtasks` (#29 in
   * `.agent/frontend-improvement-agent.log`).
   */
  const addSubmissionTokenRef = useRef(0);

  useEffect(() => {
    setChecklistModalOpen(false);
    setNewChecklistText("");
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
    addSubmissionTokenRef.current += 1;
  }, [taskId]);

  const closeChecklistModal = useCallback(() => {
    addSubmissionTokenRef.current += 1;
    setChecklistModalOpen(false);
    setNewChecklistText("");
  }, []);

  const closeEditCriterionModal = useCallback(() => {
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, []);

  const openChecklistModal = useCallback(() => {
    addSubmissionTokenRef.current += 1;
    setNewChecklistText("");
    setChecklistModalOpen(true);
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, []);

  const openEditCriterionModal = useCallback((itemId: string, text: string) => {
    addSubmissionTokenRef.current += 1;
    setEditingChecklistItemId(itemId);
    setEditChecklistText(text);
    setEditCriterionModalOpen(true);
    setChecklistModalOpen(false);
    setNewChecklistText("");
  }, []);

  const addChecklistMutation = useMutation<
    void,
    unknown,
    { text: string; submissionToken: number },
    ChecklistOptimisticContext
  >({
    mutationFn: (input) => addChecklistItem(taskId, input.text),
    onMutate: async (input) => {
      const startedAtMs = performance.now();
      rumMutationStarted("checklist_add");
      // Pessimistic path: skip the optimistic write entirely. The
      // onSuccess invalidate-and-refetch path below is still in
      // charge of surfacing the real new item once the server
      // responds, so correctness is unchanged — just no "instant"
      // render.
      if (!optimisticMutationsEnabled) {
        return { prev: undefined, startedAtMs };
      }
      await queryClient.cancelQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      const prev = snapshotChecklist(queryClient, taskId);
      const tempId = nextOptimisticChecklistId();
      const sortOrder = prev?.items.length
        ? Math.max(...prev.items.map((i) => i.sort_order)) + 1
        : 0;
      const synthetic: TaskChecklistItemView = {
        id: tempId,
        sort_order: sortOrder,
        text: input.text,
        done: false,
      };
      const next: TaskChecklistResponse = {
        items: [...(prev?.items ?? []), synthetic],
      };
      queryClient.setQueryData(taskQueryKeys.checklist(taskId), next);
      rumMutationOptimisticApplied("checklist_add", performance.now() - startedAtMs);
      return { prev, startedAtMs, tempItemId: tempId };
    },
    onError: (_err, _vars, context) => {
      if (context) {
        // Only restore + count rollback when we actually wrote
        // optimistic state. In the pessimistic branch `tempItemId`
        // is undefined, which is our marker.
        if (context.tempItemId !== undefined) {
          restoreChecklist(queryClient, taskId, context.prev);
          recordRollback("checklist_add", context.startedAtMs);
        } else {
          // Still emit a settled event so the RUM pipeline can
          // close out the mutation_started that fired in onMutate.
          rumMutationSettled(
            "checklist_add",
            performance.now() - context.startedAtMs,
            0,
          );
        }
      }
      toast.error("Couldn't add criterion - reverted.");
    },
    onSuccess: async (_item, variables, context) => {
      // Server-truth invalidations always fire: the new criterion is
      // real regardless of which modal state the user is now in.
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      // Form-clear + modal-close branch is gated on the per-submission
      // token compare so a stale resolution after the user dismissed
      // (closeChecklistModal), reopened (openChecklistModal), or
      // switched to edit mode (openEditCriterionModal) cannot clobber
      // the now-current state. Same shape as the subtask flow in #29
      // and the create / save / evaluate / resume / patch / delete
      // races hardened in #20-#26 — see
      // `.agent/frontend-improvement-agent.log`.
      if (addSubmissionTokenRef.current !== variables.submissionToken) {
        return;
      }
      setNewChecklistText("");
      setChecklistModalOpen(false);
      if (context) {
        rumMutationSettled(
          "checklist_add",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
  });

  const submitNewChecklistCriterion = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const t = newChecklistText.trim();
      if (!t || addChecklistMutation.isPending) return;
      const submissionToken = ++addSubmissionTokenRef.current;
      addChecklistMutation.mutate({ text: t, submissionToken });
    },
    [newChecklistText, addChecklistMutation],
  );

  const updateChecklistTextMutation = useMutation<
    TaskChecklistResponse,
    unknown,
    { itemId: string; text: string },
    ChecklistOptimisticContext
  >({
    mutationFn: (input) =>
      patchChecklistItemText(taskId, input.itemId, input.text),
    onMutate: async (input) => {
      const startedAtMs = performance.now();
      rumMutationStarted("checklist_edit");
      if (!optimisticMutationsEnabled) {
        return { prev: undefined, startedAtMs };
      }
      await queryClient.cancelQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      const prev = snapshotChecklist(queryClient, taskId);
      if (prev) {
        const next: TaskChecklistResponse = {
          items: prev.items.map((it) =>
            it.id === input.itemId ? { ...it, text: input.text } : it,
          ),
        };
        queryClient.setQueryData(taskQueryKeys.checklist(taskId), next);
      }
      rumMutationOptimisticApplied("checklist_edit", performance.now() - startedAtMs);
      return { prev, startedAtMs };
    },
    onError: (_err, _vars, context) => {
      if (context) {
        if (context.prev !== undefined) {
          restoreChecklist(queryClient, taskId, context.prev);
          recordRollback("checklist_edit", context.startedAtMs);
        } else {
          rumMutationSettled(
            "checklist_edit",
            performance.now() - context.startedAtMs,
            0,
          );
        }
      }
      toast.error("Couldn't update criterion - reverted.");
    },
    onSuccess: async (_item, variables, context) => {
      // Server-truth invalidations always fire: the criterion text was
      // persisted regardless of which item the user is now editing.
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      // Modal-close branch is gated on the natural id-aware compare:
      // `editingChecklistItemId` already moves on
      // `openEditCriterionModal(nextId, ...)` and clears on
      // `closeEditCriterionModal`, so a stale PATCH for criterion A
      // resolving while the user has reopened the edit modal on
      // criterion B (or dismissed entirely) won't slam B's modal shut
      // and reset its text. Same id-aware compare shape as
      // `useTaskPatchFlow` (#20) / `useTaskDeleteFlow` (#21).
      if (editingChecklistItemId !== variables.itemId) {
        return;
      }
      closeEditCriterionModal();
      if (context) {
        rumMutationSettled(
          "checklist_edit",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
  });

  const submitEditChecklistCriterion = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const t = editChecklistText.trim();
      const id = editingChecklistItemId;
      if (!t || !id || updateChecklistTextMutation.isPending) return;
      updateChecklistTextMutation.mutate({ itemId: id, text: t });
    },
    [editChecklistText, editingChecklistItemId, updateChecklistTextMutation],
  );

  const deleteChecklistMutation = useMutation<
    void,
    unknown,
    string,
    ChecklistOptimisticContext
  >({
    mutationFn: (itemId) => deleteChecklistItem(taskId, itemId),
    onMutate: async (itemId) => {
      const startedAtMs = performance.now();
      rumMutationStarted("checklist_delete");
      if (!optimisticMutationsEnabled) {
        return { prev: undefined, startedAtMs };
      }
      await queryClient.cancelQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      const prev = snapshotChecklist(queryClient, taskId);
      if (prev) {
        const next: TaskChecklistResponse = {
          items: prev.items.filter((it) => it.id !== itemId),
        };
        queryClient.setQueryData(taskQueryKeys.checklist(taskId), next);
      }
      rumMutationOptimisticApplied("checklist_delete", performance.now() - startedAtMs);
      return { prev, startedAtMs };
    },
    onError: (_err, _vars, context) => {
      if (context) {
        if (context.prev !== undefined) {
          restoreChecklist(queryClient, taskId, context.prev);
          recordRollback("checklist_delete", context.startedAtMs);
        } else {
          rumMutationSettled(
            "checklist_delete",
            performance.now() - context.startedAtMs,
            0,
          );
        }
      }
      toast.error("Couldn't delete criterion - reverted.");
    },
    onSuccess: async (_data, _itemId, context) => {
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      // The original deleteChecklistMutation only invalidated the
      // checklist query — but the task detail caches a derived
      // checklist count / summary that goes stale on delete. Add the
      // detail invalidation here so the parent task page row + summary
      // re-render after a checklist delete (the plan calls this out
      // explicitly in Phase 1d).
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      if (context) {
        rumMutationSettled(
          "checklist_delete",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
  });

  return {
    checklistModalOpen,
    newChecklistText,
    setNewChecklistText,
    editCriterionModalOpen,
    editingChecklistItemId,
    editChecklistText,
    setEditChecklistText,
    closeChecklistModal,
    closeEditCriterionModal,
    openChecklistModal,
    openEditCriterionModal,
    addChecklistMutation,
    submitNewChecklistCriterion,
    updateChecklistTextMutation,
    submitEditChecklistCriterion,
    deleteChecklistMutation,
  };
}
