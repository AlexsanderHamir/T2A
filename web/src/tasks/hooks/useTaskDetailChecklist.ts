import { useMutation, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  deleteChecklistItem,
  patchChecklistItemText,
} from "@/api";
import { taskQueryKeys } from "../task-query";

export function useTaskDetailChecklist(taskId: string, queryClient: QueryClient) {
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

  const addChecklistMutation = useMutation({
    mutationFn: (input: { text: string; submissionToken: number }) =>
      addChecklistItem(taskId, input.text),
    onSuccess: async (_item, variables) => {
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

  const updateChecklistTextMutation = useMutation({
    mutationFn: (input: { itemId: string; text: string }) =>
      patchChecklistItemText(taskId, input.itemId, input.text),
    onSuccess: async (_item, variables) => {
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

  const deleteChecklistMutation = useMutation({
    mutationFn: (itemId: string) => deleteChecklistItem(taskId, itemId),
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
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
