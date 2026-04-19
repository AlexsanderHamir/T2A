import { useMutation, type QueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState, type FormEvent } from "react";
import { addChecklistItem, createTask } from "@/api";
import {
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type PriorityChoice,
  type Task,
  type TaskType,
} from "@/types";
import { taskQueryKeys } from "../task-query";

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

  /**
   * Monotonically-increasing token used to defend `createSubtaskMutation`
   * against stale resolutions. The subtask doesn't have a stable
   * user-visible id pre-create (the parent `taskId` is the same across
   * every submission), so we use a per-submission counter as the
   * "this is the create the user is currently waiting on" identity.
   *
   * Incremented (a) on every `submitNewSubtask` call so each in-flight
   * mutation carries its own snapshot, AND (b) inside `resetSubtaskForm`
   * so any user-initiated close / open / taskId-change / fresh form
   * supersedes any in-flight create — without that second clear, an
   * in-flight A's late resolution would slam closed the freshly-opened
   * modal for a different submission B that the user hasn't tried to
   * submit yet (the comparison would still match because no new submit
   * had bumped the counter). Same shape as `requestedResumeRef` from
   * #26 in `.agent/frontend-improvement-agent.log`, but a counter
   * instead of an id since the modal has no caller-supplied identity.
   */
  const submissionTokenRef = useRef(0);

  const resetSubtaskForm = useCallback(() => {
    submissionTokenRef.current += 1;
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
      /**
       * Per-submission token captured synchronously at `submitNewSubtask`
       * time and compared against `submissionTokenRef.current` in
       * `onSuccess` to detect stale resolutions. Not part of the server
       * contract — `createTask` ignores extra fields.
       */
      submissionToken: number;
    }) => {
      const parent = queryClient.getQueryData<Task>(
        taskQueryKeys.detail(taskId),
      );
      const child = await createTask({
        title: input.title,
        initial_prompt: input.initial_prompt,
        priority: input.priority,
        task_type: input.task_type,
        parent_id: taskId,
        checklist_inherit: input.checklist_inherit,
        ...(parent
          ? {
              runner: parent.runner,
              cursor_model: parent.cursor_model,
            }
          : {}),
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
    onSuccess: async (_child, variables) => {
      // Server-truth invalidations always fire: the new subtask is real
      // regardless of whether the user is still looking at the modal
      // they submitted from. The list / detail must reflect it.
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
      // Form-clear + modal-close branch is gated on the id-aware compare:
      // if the user dismissed the modal mid-flight, switched parent task,
      // or started typing a different subtask, `submissionTokenRef.current`
      // has moved past `variables.submissionToken` and we MUST NOT clobber
      // the now-current state. Same shape as the create / save / evaluate
      // / resume / patch / delete races hardened in #20-#26 — see
      // `.agent/frontend-improvement-agent.log`.
      if (submissionTokenRef.current !== variables.submissionToken) {
        return;
      }
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
      const submissionToken = ++submissionTokenRef.current;
      createSubtaskMutation.mutate({
        title: subtaskTitle.trim(),
        initial_prompt: subtaskPrompt,
        priority: subtaskPriority,
        task_type: subtaskTaskType,
        checklist_inherit: subtaskInherit,
        checklistItems: subtaskInherit ? [] : subtaskChecklistItems,
        submissionToken,
      });
    },
    [
      subtaskTitle,
      subtaskPrompt,
      subtaskPriority,
      subtaskTaskType,
      subtaskInherit,
      subtaskChecklistItems,
      createSubtaskMutation,
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
