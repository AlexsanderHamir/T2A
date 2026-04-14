import { useMutation, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState, type FormEvent } from "react";
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

  useEffect(() => {
    setChecklistModalOpen(false);
    setNewChecklistText("");
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, [taskId]);

  const closeChecklistModal = useCallback(() => {
    setChecklistModalOpen(false);
    setNewChecklistText("");
  }, []);

  const closeEditCriterionModal = useCallback(() => {
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, []);

  const openChecklistModal = useCallback(() => {
    setNewChecklistText("");
    setChecklistModalOpen(true);
    setEditCriterionModalOpen(false);
    setEditingChecklistItemId(null);
    setEditChecklistText("");
  }, []);

  const openEditCriterionModal = useCallback((itemId: string, text: string) => {
    setEditingChecklistItemId(itemId);
    setEditChecklistText(text);
    setEditCriterionModalOpen(true);
    setChecklistModalOpen(false);
    setNewChecklistText("");
  }, []);

  const addChecklistMutation = useMutation({
    mutationFn: (text: string) => addChecklistItem(taskId, text),
    onSuccess: async () => {
      setNewChecklistText("");
      setChecklistModalOpen(false);
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
    },
  });

  const submitNewChecklistCriterion = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      const t = newChecklistText.trim();
      if (!t || addChecklistMutation.isPending) return;
      addChecklistMutation.mutate(t);
    },
    [newChecklistText, addChecklistMutation],
  );

  const updateChecklistTextMutation = useMutation({
    mutationFn: (input: { itemId: string; text: string }) =>
      patchChecklistItemText(taskId, input.itemId, input.text),
    onSuccess: async () => {
      closeEditCriterionModal();
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.checklist(taskId),
      });
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
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
