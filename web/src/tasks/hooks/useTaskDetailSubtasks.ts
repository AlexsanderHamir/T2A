import { useMutation, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useState, type FormEvent } from "react";
import { addChecklistItem, createTask } from "@/api";
import {
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type PriorityChoice,
  type TaskType,
} from "@/types";
import { taskQueryKeys } from "../queryKeys";

export function useTaskDetailSubtasks(taskId: string, queryClient: QueryClient) {
  const [subtaskTitle, setSubtaskTitle] = useState("");
  const [subtaskPrompt, setSubtaskPrompt] = useState("");
  const [subtaskPriority, setSubtaskPriority] = useState<PriorityChoice>("");
  const [subtaskTaskType, setSubtaskTaskType] = useState<TaskType>(
    DEFAULT_NEW_TASK_TYPE,
  );
  const [subtaskChecklistItems, setSubtaskChecklistItems] = useState<string[]>(
    [],
  );
  const [subtaskInherit, setSubtaskInherit] = useState(false);
  const [subtaskModalOpen, setSubtaskModalOpen] = useState(false);

  const resetSubtaskForm = useCallback(() => {
    setSubtaskTitle("");
    setSubtaskPrompt("");
    setSubtaskPriority("");
    setSubtaskTaskType(DEFAULT_NEW_TASK_TYPE);
    setSubtaskChecklistItems([]);
    setSubtaskInherit(false);
  }, []);

  const closeSubtaskModal = useCallback(() => {
    setSubtaskModalOpen(false);
    resetSubtaskForm();
  }, [resetSubtaskForm]);

  const openSubtaskModal = useCallback(() => {
    resetSubtaskForm();
    setSubtaskModalOpen(true);
  }, [resetSubtaskForm]);

  useEffect(() => {
    setSubtaskModalOpen(false);
    resetSubtaskForm();
  }, [taskId, resetSubtaskForm]);

  useEffect(() => {
    if (!subtaskInherit) return;
    setSubtaskChecklistItems([]);
  }, [subtaskInherit]);

  const appendSubtaskChecklistCriterion = useCallback((raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setSubtaskChecklistItems((prev) => [...prev, t]);
  }, []);

  const removeSubtaskChecklistRow = useCallback((index: number) => {
    setSubtaskChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const updateSubtaskChecklistRow = useCallback((index: number, raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setSubtaskChecklistItems((prev) => prev.map((x, i) => (i === index ? t : x)));
  }, []);

  const createSubtaskMutation = useMutation({
    mutationFn: async (input: {
      title: string;
      initial_prompt: string;
      priority: Priority;
      task_type: TaskType;
      checklist_inherit: boolean;
      checklistItems: string[];
    }) => {
      const child = await createTask({
        title: input.title,
        initial_prompt: input.initial_prompt,
        priority: input.priority,
        task_type: input.task_type,
        parent_id: taskId,
        checklist_inherit: input.checklist_inherit,
      });
      if (!input.checklist_inherit) {
        for (const raw of input.checklistItems) {
          const text = raw.trim();
          if (text) {
            await addChecklistItem(child.id, text);
          }
        }
      }
      return child;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
      closeSubtaskModal();
    },
  });

  const submitNewSubtask = useCallback(
    (e: FormEvent) => {
      e.preventDefault();
      if (
        !subtaskTitle.trim() ||
        !subtaskPriority ||
        createSubtaskMutation.isPending
      ) {
        return;
      }
      createSubtaskMutation.mutate({
        title: subtaskTitle.trim(),
        initial_prompt: subtaskPrompt,
        priority: subtaskPriority,
        task_type: subtaskTaskType,
        checklist_inherit: subtaskInherit,
        checklistItems: subtaskInherit ? [] : subtaskChecklistItems,
      });
    },
    [
      subtaskTitle,
      subtaskPrompt,
      subtaskPriority,
      subtaskTaskType,
      subtaskInherit,
      subtaskChecklistItems,
      createSubtaskMutation.mutate,
      createSubtaskMutation.isPending,
    ],
  );

  return {
    subtaskModalOpen,
    subtaskTitle,
    setSubtaskTitle,
    subtaskPrompt,
    setSubtaskPrompt,
    subtaskPriority,
    setSubtaskPriority,
    subtaskTaskType,
    setSubtaskTaskType,
    subtaskChecklistItems,
    subtaskInherit,
    setSubtaskInherit,
    openSubtaskModal,
    closeSubtaskModal,
    appendSubtaskChecklistCriterion,
    removeSubtaskChecklistRow,
    updateSubtaskChecklistRow,
    createSubtaskMutation,
    submitNewSubtask,
  };
}
