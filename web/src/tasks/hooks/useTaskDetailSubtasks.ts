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
import {
  rumMutationOptimisticApplied,
  rumMutationRolledBack,
  rumMutationSettled,
  rumMutationStarted,
} from "@/observability";
import { useOptionalToast } from "@/shared/toast";
import { useRolloutFlags } from "@/settings";
import { taskQueryKeys } from "../task-query";
import { bumpOptimisticVersion, clearOptimisticVersion } from "./optimisticVersion";

let optimisticSubtaskCounter = 0;
function nextOptimisticSubtaskId(): string {
  optimisticSubtaskCounter += 1;
  return `optimistic-subtask-${optimisticSubtaskCounter}`;
}

type SubtaskOptimisticContext = {
  prevParent: Task | undefined;
  optimisticId: string;
  startedAtMs: number;
};

export function useTaskDetailSubtasks(taskId: string, queryClient: QueryClient) {
  const { optimisticMutationsEnabled } = useRolloutFlags();
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

  const toast = useOptionalToast();
  const createSubtaskMutation = useMutation<
    Task,
    unknown,
    {
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
    },
    SubtaskOptimisticContext
  >({
    mutationFn: async (input) => {
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
    onMutate: async (input) => {
      const startedAtMs = performance.now();
      rumMutationStarted("subtask_create");
      if (!optimisticMutationsEnabled) {
        return { prevParent: undefined, optimisticId: "", startedAtMs };
      }
      bumpOptimisticVersion(taskId);
      await queryClient.cancelQueries({ queryKey: taskQueryKeys.detail(taskId) });
      const detailKey = taskQueryKeys.detail(taskId);
      const prevParent = queryClient.getQueryData<Task>(detailKey);
      const optimisticId = nextOptimisticSubtaskId();
      if (prevParent) {
        const synthetic: Task = {
          id: optimisticId,
          title: input.title,
          initial_prompt: input.initial_prompt,
          status: "ready",
          priority: input.priority,
          task_type: input.task_type,
          runner: prevParent.runner,
          cursor_model: prevParent.cursor_model,
          parent_id: taskId,
          checklist_inherit: input.checklist_inherit,
          children: [],
        };
        queryClient.setQueryData<Task>(detailKey, {
          ...prevParent,
          children: [...(prevParent.children ?? []), synthetic],
        });
      }
      rumMutationOptimisticApplied(
        "subtask_create",
        performance.now() - startedAtMs,
      );
      return { prevParent, optimisticId, startedAtMs };
    },
    onError: (_err, _vars, context) => {
      if (context?.prevParent) {
        queryClient.setQueryData(
          taskQueryKeys.detail(taskId),
          context.prevParent,
        );
      }
      if (context) {
        // Only mark this as a rollback in RUM when we actually
        // wrote optimistic state; in the pessimistic branch the
        // UI was never updated so there's nothing to count. We
        // still settle so mutation_started has a close-out.
        if (context.optimisticId !== "") {
          rumMutationRolledBack(
            "subtask_create",
            performance.now() - context.startedAtMs,
          );
        }
        rumMutationSettled(
          "subtask_create",
          performance.now() - context.startedAtMs,
          0,
        );
      }
      toast.error("Couldn't create subtask - reverted.");
    },
    onSuccess: async (_child, variables, context) => {
      await queryClient.invalidateQueries({
        queryKey: taskQueryKeys.detail(taskId),
      });
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
      if (context) {
        rumMutationSettled(
          "subtask_create",
          performance.now() - context.startedAtMs,
          200,
        );
      }
      if (submissionTokenRef.current !== variables.submissionToken) {
        return;
      }
      closeSubtaskModal();
    },
    onSettled: () => {
      clearOptimisticVersion(taskId);
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
