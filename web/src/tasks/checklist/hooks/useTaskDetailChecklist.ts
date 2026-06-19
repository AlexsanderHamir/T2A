import { useMutation, useQueryClient, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  deleteChecklistItem,
  patchChecklistItemText,
  patchChecklistItemVerifyCommands,
} from "@/api";
import {
  normalizeVerifyCommands,
} from "@/tasks/task-compose/checklistRequirement";
import type { ChecklistVerifyCommandInput } from "@/types";
import {
  rumMutationRolledBack,
  rumMutationSettled,
  type RUMMutationKind,
} from "@/observability";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import {
  beginGuardedTaskWrite,
  endGuardedTaskWrite,
  recordOptimisticApplied,
} from "@/tasks/mutations";
import { taskQueryKeys } from "@/tasks/task-query";
import type { TaskChecklistItemView, TaskChecklistResponse } from "@/types";

interface ChecklistOptimisticContext {
  prev: TaskChecklistResponse | undefined;
  startedAtMs: number;
  guarded: boolean;
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
  const [newChecklistVerifyCommands, setNewChecklistVerifyCommands] = useState<
    ChecklistVerifyCommandInput[]
  >([]);
  const [editCriterionModalOpen, setEditCriterionModalOpen] = useState(false);
  const [editingChecklistItemId, setEditingChecklistItemId] = useState<
    string | null
  >(null);
  const [editChecklistText, setEditChecklistText] = useState("");
  const [editChecklistVerifyCommands, setEditChecklistVerifyCommands] = useState<
    ChecklistVerifyCommandInput[]
  >([]);
  /**
   * Stored at edit-open time so `submitEditChecklistCriterion` can skip
   * a no-op PATCH when the user saved without changing the text.
   */
  const [editChecklistOriginalText, setEditChecklistOriginalText] =
    useState("");
  const [editChecklistOriginalVerifyCommands, setEditChecklistOriginalVerifyCommands] =
    useState<ChecklistVerifyCommandInput[]>([]);

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

  /**
   * Tracks the live `editingChecklistItemId` for the async
   * `submitEditChecklistCriterion` flow. The submit awaits the PATCH
   * `mutateAsync` promise, and between the await and the close the
   * user may have reopened the edit modal on a different item or
   * dismissed entirely; reading state via this ref captures the latest
   * value rather than the closure-captured value at submit time.
   */
  const editingChecklistItemIdRef = useRef<string | null>(null);
  useEffect(() => {
    editingChecklistItemIdRef.current = editingChecklistItemId;
  }, [editingChecklistItemId]);

  useEffect(() => {
    setChecklistModalOpen(false);
    setNewChecklistText("");
    setNewChecklistVerifyCommands([]);
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
    setEditChecklistOriginalText("");
    setEditChecklistVerifyCommands([]);
    setEditChecklistOriginalVerifyCommands([]);
    addSubmissionTokenRef.current += 1;
  }, [taskId]);

  const closeChecklistModal = useCallback(() => {
    addSubmissionTokenRef.current += 1;
    setChecklistModalOpen(false);
    setNewChecklistText("");
    setNewChecklistVerifyCommands([]);
  }, []);

  const closeEditCriterionModal = useCallback(() => {
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
    setEditChecklistOriginalText("");
    setEditChecklistVerifyCommands([]);
    setEditChecklistOriginalVerifyCommands([]);
  }, []);

  const openChecklistModal = useCallback(() => {
    addSubmissionTokenRef.current += 1;
    setNewChecklistText("");
    setNewChecklistVerifyCommands([]);
    setChecklistModalOpen(true);
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
    setEditChecklistOriginalText("");
    setEditChecklistVerifyCommands([]);
    setEditChecklistOriginalVerifyCommands([]);
  }, []);

  const openEditCriterionModal = useCallback(
    (itemId: string, text: string, verifyCommands: ChecklistVerifyCommandInput[] = []) => {
      addSubmissionTokenRef.current += 1;
      setEditingChecklistItemId(itemId);
      setEditChecklistText(text);
      setEditChecklistOriginalText(text);
      const cmds = verifyCommands ?? [];
      setEditChecklistVerifyCommands(cmds);
      setEditChecklistOriginalVerifyCommands(cmds);
      setEditCriterionModalOpen(true);
      setChecklistModalOpen(false);
      setNewChecklistText("");
      setNewChecklistVerifyCommands([]);
    },
    [],
  );

  const addChecklistMutation = useMutation<
    void,
    unknown,
    { text: string; verify_commands: ChecklistVerifyCommandInput[]; submissionToken: number },
    ChecklistOptimisticContext
  >({
    mutationFn: (input) =>
      addChecklistItem(taskId, input.text, {
        verify_commands: input.verify_commands,
      }),
    onMutate: async (input) => {
      const guard = beginGuardedTaskWrite({
        taskId,
        optimisticEnabled: optimisticMutationsEnabled,
        rumKind: "checklist_add",
      });
      if (!guard.guarded) {
        return { prev: undefined, startedAtMs: guard.startedAtMs, guarded: false };
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
      recordOptimisticApplied("checklist_add", guard.startedAtMs);
      return { prev, startedAtMs: guard.startedAtMs, tempItemId: tempId, guarded: true };
    },
    onError: (_err, _vars, context) => {
      if (context) {
        // Only restore + count rollback when we actually wrote
        // optimistic state. In the pessimistic branch `tempItemId`
        // is undefined, which is our marker.
        if (context.guarded && context.tempItemId !== undefined) {
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
      setNewChecklistVerifyCommands([]);
      setChecklistModalOpen(false);
      if (context) {
        rumMutationSettled(
          "checklist_add",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: (_data, _err, _vars, context) => {
      if (context?.guarded) {
        endGuardedTaskWrite(taskId);
      }
    },
  });

  const submitNewChecklistCriterion = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const t = newChecklistText.trim();
      if (!t || addChecklistMutation.isPending) return;
      const submissionToken = ++addSubmissionTokenRef.current;
      const verify_commands = normalizeVerifyCommands(newChecklistVerifyCommands);
      addChecklistMutation.mutate({ text: t, verify_commands, submissionToken });
    },
    [newChecklistText, newChecklistVerifyCommands, addChecklistMutation],
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
      const guard = beginGuardedTaskWrite({
        taskId,
        optimisticEnabled: optimisticMutationsEnabled,
        rumKind: "checklist_edit",
      });
      if (!guard.guarded) {
        return { prev: undefined, startedAtMs: guard.startedAtMs, guarded: false };
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
      recordOptimisticApplied("checklist_edit", guard.startedAtMs);
      return { prev, startedAtMs: guard.startedAtMs, guarded: true };
    },
    onError: (_err, _vars, context) => {
      if (context) {
        if (context.guarded && context.prev !== undefined) {
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
    onSuccess: async (_item, _variables, context) => {
      // Server-truth invalidations always fire: the criterion text was
      // persisted regardless of which item the user is now editing.
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      if (context) {
        rumMutationSettled(
          "checklist_edit",
          performance.now() - context.startedAtMs,
          200,
        );
      }
    },
    onSettled: (_data, _err, _vars, context) => {
      if (context?.guarded) {
        endGuardedTaskWrite(taskId);
      }
    },
  });

  const submitEditChecklistCriterion = useCallback(
    async (e: FormEvent) => {
      e.preventDefault();
      const id = editingChecklistItemId;
      if (!id) return;
      const newText = editChecklistText.trim();
      if (!newText) return;
      if (updateChecklistTextMutation.isPending) return;
      const newCommands = normalizeVerifyCommands(editChecklistVerifyCommands);
      const textChanged = newText !== editChecklistOriginalText;
      const commandsChanged =
        JSON.stringify(newCommands) !==
        JSON.stringify(normalizeVerifyCommands(editChecklistOriginalVerifyCommands));
      if (!textChanged && !commandsChanged) {
        closeEditCriterionModal();
        return;
      }
      try {
        if (textChanged) {
          await updateChecklistTextMutation.mutateAsync({
            itemId: id,
            text: newText,
          });
        }
        if (commandsChanged) {
          await patchChecklistItemVerifyCommands(taskId, id, newCommands);
          await queryClient.invalidateQueries({
            queryKey: taskQueryKeys.checklist(taskId),
          });
        }
        if (editingChecklistItemIdRef.current === id) {
          closeEditCriterionModal();
        }
      } catch {
        // mutateAsync rejects when the underlying mutation errors. The
        // mutation's own `error` is now populated and the modal stays
        // open so `MutationErrorBanner` can surface it.
      }
    },
    [
      editingChecklistItemId,
      editChecklistText,
      editChecklistOriginalText,
      editChecklistVerifyCommands,
      editChecklistOriginalVerifyCommands,
      updateChecklistTextMutation,
      closeEditCriterionModal,
      queryClient,
      taskId,
    ],
  );

  const deleteChecklistMutation = useMutation<
    void,
    unknown,
    string,
    ChecklistOptimisticContext
  >({
    mutationFn: (itemId) => deleteChecklistItem(taskId, itemId),
    onMutate: async (itemId) => {
      const guard = beginGuardedTaskWrite({
        taskId,
        optimisticEnabled: optimisticMutationsEnabled,
        rumKind: "checklist_delete",
      });
      if (!guard.guarded) {
        return { prev: undefined, startedAtMs: guard.startedAtMs, guarded: false };
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
      recordOptimisticApplied("checklist_delete", guard.startedAtMs);
      return { prev, startedAtMs: guard.startedAtMs, guarded: true };
    },
    onError: (_err, _vars, context) => {
      if (context) {
        if (context.guarded && context.prev !== undefined) {
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
    onSettled: (_data, _err, _vars, context) => {
      if (context?.guarded) {
        endGuardedTaskWrite(taskId);
      }
    },
  });

  return {
    checklistModalOpen,
    newChecklistText,
    setNewChecklistText,
    newChecklistVerifyCommands,
    setNewChecklistVerifyCommands,
    editCriterionModalOpen,
    editingChecklistItemId,
    editChecklistText,
    setEditChecklistText,
    editChecklistVerifyCommands,
    setEditChecklistVerifyCommands,
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
