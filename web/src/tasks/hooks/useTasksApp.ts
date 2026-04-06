import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  createTask as apiCreate,
  deleteTaskDraft as apiDeleteDraft,
  deleteTask as apiDelete,
  evaluateDraftTask as apiEvaluateDraft,
  getTaskDraft as apiGetDraft,
  listTaskDrafts as apiListDrafts,
  listTasks,
  patchTask,
  saveTaskDraft as apiSaveDraft,
} from "../../api";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { flattenTaskTree, flattenTaskTreeRoots } from "../flattenTaskTree";
import { TASK_LIST_PAGE_SIZE } from "../paging";
import { taskQueryKeys } from "../queryKeys";
import {
  DEFAULT_NEW_TASK_STATUS,
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type PriorityChoice,
  type Status,
  type Task,
  type TaskType,
} from "@/types";
import { useHysteresisBoolean } from "@/lib/useHysteresisBoolean";
import { useTaskEventStream } from "./useTaskEventStream";

/** Background refetches (SSE invalidate, focus) are short; avoid UI flicker. */
const LIST_REFRESH_SHOW_MS = 380;
const LIST_REFRESH_HIDE_MS = 520;
const DRAFT_AUTOSAVE_DEBOUNCE_MS = 900;

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export function useTasksApp() {
  const queryClient = useQueryClient();
  const sseLive = useTaskEventStream();

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newPriority, setNewPriority] = useState<PriorityChoice>("");
  const [newTaskType, setNewTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [newChecklistItems, setNewChecklistItems] = useState<string[]>([]);
  const [newDraftID, setNewDraftID] = useState("");
  const [newDraftName, setNewDraftName] = useState("");
  const [draftPickerOpen, setDraftPickerOpen] = useState(false);
  const [latestDraftEvaluation, setLatestDraftEvaluation] = useState<{
    overallScore: number;
    overallSummary: string;
    sections: Array<{ key: string; score: number }>;
  } | null>(null);
  /** Child tasks (full draft) created after the parent task on the home flow. */
  const [pendingSubtasks, setPendingSubtasks] = useState<PendingSubtaskDraft[]>(
    [],
  );
  /** When set, POST /tasks includes `parent_id` (subtask on the home page). */
  const [newParentId, setNewParentId] = useState("");
  const [newChecklistInherit, setNewChecklistInherit] = useState(false);
  const [createModalOpen, setCreateModalOpen] = useState(false);

  const [editing, setEditing] = useState<Task | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editPriority, setEditPriority] = useState<Priority>("medium");
  const [editTaskType, setEditTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [editStatus, setEditStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [editChecklistInherit, setEditChecklistInherit] = useState(false);

  /** In-app delete confirmation (avoids `window.confirm`, which breaks input focus in some browsers). */
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    title: string;
    parent_id?: string;
  } | null>(null);

  /** Client-side validation (shown after server errors when applicable). */
  const [editTitleRequiredError, setEditTitleRequiredError] = useState<
    string | null
  >(null);

  const [taskListPage, setTaskListPage] = useState(0);

  const tasksQuery = useQuery({
    queryKey: taskQueryKeys.list(taskListPage),
    queryFn: ({ signal }) =>
      listTasks(
        TASK_LIST_PAGE_SIZE,
        taskListPage * TASK_LIST_PAGE_SIZE,
        { signal },
      ),
  });
  const draftsQuery = useQuery({
    queryKey: ["task-drafts"],
    queryFn: ({ signal }) => apiListDrafts(100, { signal }),
  });

  const resetTaskListPage = useCallback(() => {
    setTaskListPage(0);
  }, []);

  const rootTaskTrees = tasksQuery.data?.tasks ?? [];
  const tasks = useMemo(
    () => flattenTaskTreeRoots(rootTaskTrees),
    [rootTaskTrees],
  );
  const parentPickerTasks = useMemo(
    () => flattenTaskTree(rootTaskTrees),
    [rootTaskTrees],
  );

  useEffect(() => {
    if (!newParentId) {
      setNewChecklistInherit(false);
    }
  }, [newParentId]);

  useEffect(() => {
    if (!newChecklistInherit) return;
    setNewChecklistItems([]);
  }, [newChecklistInherit]);

  const resetNewTaskForm = useCallback(() => {
    const generatedID =
      typeof crypto !== "undefined" && typeof crypto.randomUUID === "function"
        ? crypto.randomUUID()
        : `draft-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 8)}`;
    setNewTitle("");
    setNewPrompt("");
    setNewPriority("");
    setNewTaskType(DEFAULT_NEW_TASK_TYPE);
    setNewChecklistItems([]);
    setPendingSubtasks([]);
    setNewParentId("");
    setNewChecklistInherit(false);
    setLatestDraftEvaluation(null);
    setNewDraftID(generatedID);
    setNewDraftName("Untitled draft");
  }, []);

  const closeCreateModal = useCallback(() => {
    setCreateModalOpen(false);
    setDraftPickerOpen(false);
    resetNewTaskForm();
  }, [resetNewTaskForm]);

  const openCreateModal = useCallback(() => {
    const drafts = draftsQuery.data ?? [];
    if (drafts.length > 0) {
      setDraftPickerOpen(true);
      return;
    }
    resetNewTaskForm();
    setCreateModalOpen(true);
  }, [draftsQuery.data, resetNewTaskForm]);

  const loading = tasksQuery.isPending;
  const rawListRefreshing =
    tasksQuery.isFetching && !tasksQuery.isPending;
  const listRefreshing = useHysteresisBoolean(
    rawListRefreshing,
    LIST_REFRESH_SHOW_MS,
    LIST_REFRESH_HIDE_MS,
  );

  const createMutation = useMutation({
    mutationFn: async (input: {
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      task_type: TaskType;
      parent_id?: string;
      checklist_inherit: boolean;
      checklistItems: string[];
      pendingSubtasks: PendingSubtaskDraft[];
      draft_id: string;
    }) => {
      const parentId = input.parent_id?.trim();
      const inherit =
        Boolean(parentId) && input.checklist_inherit === true;
      const task = await apiCreate({
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        task_type: input.task_type,
        draft_id: input.draft_id,
        ...(parentId ? { parent_id: parentId } : {}),
        ...(inherit ? { checklist_inherit: true } : {}),
      });
      if (!inherit) {
        for (const raw of input.checklistItems) {
          const text = raw.trim();
          if (text) {
            await addChecklistItem(task.id, text);
          }
        }
      }
      for (const st of input.pendingSubtasks) {
        if (!st.title.trim()) continue;
        const childInherit = st.checklist_inherit === true;
        const child = await apiCreate({
          title: st.title.trim(),
          initial_prompt: st.initial_prompt,
          status: input.status,
          priority: st.priority,
          task_type: st.task_type,
          parent_id: task.id,
          ...(childInherit ? { checklist_inherit: true } : {}),
        });
        if (!childInherit) {
          for (const raw of st.checklistItems) {
            const text = raw.trim();
            if (text) {
              await addChecklistItem(child.id, text);
            }
          }
        }
      }
      return task;
    },
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
      closeCreateModal();
    },
  });

  const evaluateDraftMutation = useMutation({
    mutationFn: async (input: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      task_type: TaskType;
      parent_id?: string;
      checklist_inherit: boolean;
      checklistItems: string[];
    }) => {
      const parentId = input.parent_id?.trim();
      return apiEvaluateDraft({
        id: input.id,
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
        task_type: input.task_type,
        ...(parentId ? { parent_id: parentId } : {}),
        ...(parentId ? { checklist_inherit: input.checklist_inherit } : {}),
        checklist_items: input.checklistItems
          .map((text) => text.trim())
          .filter(Boolean)
          .map((text) => ({ text })),
      });
    },
    onSuccess: (evaluation) => {
      setLatestDraftEvaluation({
        overallScore: evaluation.overall_score,
        overallSummary: evaluation.overall_summary,
        sections: evaluation.sections.map((s) => ({ key: s.key, score: s.score })),
      });
    },
  });

  const patchMutation = useMutation({
    mutationFn: (args: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      task_type: TaskType;
      checklist_inherit: boolean;
    }) =>
      patchTask(args.id, {
        title: args.title,
        initial_prompt: args.initial_prompt,
        status: args.status,
        priority: args.priority,
        task_type: args.task_type,
        checklist_inherit: args.checklist_inherit,
      }),
    onSuccess: async () => {
      setEditing(null);
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (input: { id: string; parent_id?: string }) =>
      apiDelete(input.id),
    onSuccess: async (_, variables) => {
      const deletedId = variables.id;
      setDeleteTarget(null);
      setEditing((prev) => (prev?.id === deletedId ? null : prev));
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
    },
  });

  const saveDraftMutation = useMutation({
    mutationFn: (input: {
      id: string;
      name: string;
      payload: {
        title: string;
        initial_prompt: string;
        priority: PriorityChoice;
        task_type: TaskType;
        parent_id: string;
        checklist_inherit: boolean;
        checklist_items: string[];
        pending_subtasks: Array<{
          title: string;
          initial_prompt: string;
          priority: Priority;
          task_type: TaskType;
          checklist_items: string[];
          checklist_inherit: boolean;
        }>;
        latest_evaluation?: {
          overall_score: number;
          overall_summary: string;
          sections: Array<{ key: string; score: number }>;
        };
      };
    }) => apiSaveDraft(input),
    onSuccess: async (saved) => {
      if (saved.id !== newDraftID) {
        setNewDraftID(saved.id);
      }
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
    },
  });

  const deleteDraftMutation = useMutation({
    mutationFn: (id: string) => apiDeleteDraft(id),
    onSuccess: async () => {
      await queryClient.invalidateQueries({ queryKey: ["task-drafts"] });
    },
  });

  const saving =
    createMutation.isPending ||
    evaluateDraftMutation.isPending ||
    patchMutation.isPending ||
    deleteMutation.isPending;

  const error = useMemo(() => {
    if (tasksQuery.isError) return errorMessage(tasksQuery.error);
    if (createMutation.isError) return errorMessage(createMutation.error);
    if (evaluateDraftMutation.isError)
      return errorMessage(evaluateDraftMutation.error);
    if (patchMutation.isError) return errorMessage(patchMutation.error);
    if (deleteMutation.isError) return errorMessage(deleteMutation.error);
    return editTitleRequiredError;
  }, [
    tasksQuery.isError,
    tasksQuery.error,
    createMutation.isError,
    createMutation.error,
    evaluateDraftMutation.isError,
    evaluateDraftMutation.error,
    patchMutation.isError,
    patchMutation.error,
    deleteMutation.isError,
    deleteMutation.error,
    editTitleRequiredError,
  ]);

  useEffect(() => {
    if (editTitleRequiredError && editTitle.trim()) {
      setEditTitleRequiredError(null);
    }
  }, [editTitle, editTitleRequiredError]);

  const hasDraftContent =
    Boolean(newTitle.trim()) ||
    Boolean(newPrompt.trim()) ||
    Boolean(newPriority) ||
    Boolean(newParentId.trim()) ||
    newChecklistItems.length > 0 ||
    pendingSubtasks.length > 0;

  useEffect(() => {
    if (!createModalOpen || !newDraftID || !hasDraftContent) return;
    const t = setTimeout(() => {
      saveDraftMutation.mutate({
        id: newDraftID,
        name: newDraftName.trim() || "Untitled draft",
        payload: {
          title: newTitle,
          initial_prompt: newPrompt,
          priority: newPriority,
          task_type: newTaskType,
          parent_id: newParentId,
          checklist_inherit: newChecklistInherit,
          checklist_items: newChecklistItems,
          pending_subtasks: pendingSubtasks.map((st) => ({
            title: st.title,
            initial_prompt: st.initial_prompt,
            priority: st.priority,
            task_type: st.task_type,
            checklist_items: st.checklistItems,
            checklist_inherit: st.checklist_inherit,
          })),
          ...(latestDraftEvaluation
            ? {
                latest_evaluation: {
                  overall_score: latestDraftEvaluation.overallScore,
                  overall_summary: latestDraftEvaluation.overallSummary,
                  sections: latestDraftEvaluation.sections,
                },
              }
            : {}),
        },
      });
    }, DRAFT_AUTOSAVE_DEBOUNCE_MS);
    return () => clearTimeout(t);
  }, [
    createModalOpen,
    hasDraftContent,
    latestDraftEvaluation,
    newChecklistInherit,
    newChecklistItems,
    newDraftID,
    newDraftName,
    newParentId,
    newPriority,
    newPrompt,
    newTaskType,
    newTitle,
    pendingSubtasks,
    saveDraftMutation,
  ]);

  async function evaluateDraftBeforeCreate() {
    const parentId = newParentId.trim();
    if (!newTitle.trim() || !newPriority) return;
    await evaluateDraftMutation.mutateAsync({
      id: newDraftID,
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
      task_type: newTaskType,
      ...(parentId ? { parent_id: parentId } : {}),
      checklist_inherit: Boolean(parentId) && newChecklistInherit,
      checklistItems: newChecklistItems,
    });
  }

  async function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!newTitle.trim() || !newPriority) return;
    const parentId = newParentId.trim();
    createMutation.mutate({
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
      task_type: newTaskType,
      draft_id: newDraftID,
      ...(parentId ? { parent_id: parentId } : {}),
      checklist_inherit: Boolean(parentId) && newChecklistInherit,
      checklistItems: newChecklistItems,
      pendingSubtasks,
    });
  }

  async function startFreshDraft() {
    resetNewTaskForm();
    setDraftPickerOpen(false);
    setCreateModalOpen(true);
  }

  async function resumeDraftByID(id: string) {
    const draft = await apiGetDraft(id);
    setNewDraftID(draft.id);
    setNewDraftName(draft.name);
    setNewTitle(draft.payload.title ?? "");
    setNewPrompt(draft.payload.initial_prompt ?? "");
    setNewPriority(draft.payload.priority ?? "");
    setNewTaskType(draft.payload.task_type ?? DEFAULT_NEW_TASK_TYPE);
    setNewParentId(draft.payload.parent_id ?? "");
    setNewChecklistInherit(draft.payload.checklist_inherit === true);
    setNewChecklistItems(draft.payload.checklist_items ?? []);
    setPendingSubtasks(
      (draft.payload.pending_subtasks ?? []).map((st) => ({
        title: st.title,
        initial_prompt: st.initial_prompt,
        priority: st.priority,
        task_type: st.task_type,
        checklistItems: st.checklist_items,
        checklist_inherit: st.checklist_inherit,
      })),
    );
    setLatestDraftEvaluation(
      draft.payload.latest_evaluation
        ? {
            overallScore: draft.payload.latest_evaluation.overall_score,
            overallSummary: draft.payload.latest_evaluation.overall_summary,
            sections: draft.payload.latest_evaluation.sections,
          }
        : null,
    );
    setDraftPickerOpen(false);
    setCreateModalOpen(true);
  }

  async function deleteDraftByID(id: string) {
    await deleteDraftMutation.mutateAsync(id);
  }

  const appendNewChecklistCriterion = useCallback((raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setNewChecklistItems((prev) => [...prev, t]);
  }, []);

  const removeNewChecklistRow = useCallback((index: number) => {
    setNewChecklistItems((prev) => prev.filter((_, i) => i !== index));
  }, []);

  const updateNewChecklistRow = useCallback((index: number, raw: string) => {
    const t = raw.trim();
    if (!t) return;
    setNewChecklistItems((prev) => prev.map((x, i) => (i === index ? t : x)));
  }, []);

  const addPendingSubtask = useCallback((d: PendingSubtaskDraft) => {
    setPendingSubtasks((prev) => [...prev, d]);
  }, []);

  const updatePendingSubtask = useCallback(
    (index: number, d: PendingSubtaskDraft) => {
      setPendingSubtasks((prev) =>
        prev.map((x, i) => (i === index ? d : x)),
      );
    },
    [],
  );

  const removePendingSubtask = useCallback((index: number) => {
    setPendingSubtasks((prev) => prev.filter((_, i) => i !== index));
  }, []);

  function openEdit(t: Task) {
    setEditing(t);
    setEditTitle(t.title);
    setEditPrompt(t.initial_prompt);
    setEditPriority(t.priority);
    setEditTaskType(t.task_type ?? DEFAULT_NEW_TASK_TYPE);
    setEditStatus(t.status);
    setEditChecklistInherit(t.checklist_inherit === true);
    setEditTitleRequiredError(null);
  }

  function closeEdit() {
    setEditing(null);
    setEditTitleRequiredError(null);
  }

  function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editing) return;
    if (!editTitle.trim()) {
      setEditTitleRequiredError("Title is required.");
      return;
    }
    setEditTitleRequiredError(null);
    patchMutation.mutate({
      id: editing.id,
      title: editTitle.trim(),
      initial_prompt: editPrompt,
      status: editStatus,
      priority: editPriority,
      task_type: editTaskType,
      checklist_inherit: editChecklistInherit,
    });
  }

  const requestDelete = useCallback((t: Task) => {
    const pid = t.parent_id?.trim();
    setDeleteTarget({
      id: t.id,
      title: t.title,
      ...(pid ? { parent_id: pid } : {}),
    });
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  function confirmDelete() {
    if (!deleteTarget) return;
    deleteMutation.mutate({
      id: deleteTarget.id,
      ...(deleteTarget.parent_id
        ? { parent_id: deleteTarget.parent_id }
        : {}),
    });
  }

  const createPending = createMutation.isPending;
  const evaluatePending = evaluateDraftMutation.isPending;
  const patchPending = patchMutation.isPending;
  const deletePending = deleteMutation.isPending;

  useEffect(() => {
    if (!tasksQuery.isPending && rootTaskTrees.length === 0 && taskListPage > 0) {
      setTaskListPage(0);
    }
  }, [tasksQuery.isPending, rootTaskTrees.length, taskListPage]);

  const hasNextTaskPage = rootTaskTrees.length === TASK_LIST_PAGE_SIZE;
  const hasPrevTaskPage = taskListPage > 0;

  return {
    tasks,
    parentPickerTasks,
    rootTasksOnPage: rootTaskTrees.length,
    loading,
    listRefreshing,
    saving,
    createPending,
    evaluatePending,
    patchPending,
    deletePending,
    deleteMutation,
    error,
    sseLive,
    draftPickerOpen,
    setDraftPickerOpen,
    taskDrafts: draftsQuery.data ?? [],
    newDraftName,
    setNewDraftName,
    newTitle,
    setNewTitle,
    newPrompt,
    setNewPrompt,
    newPriority,
    newTaskType,
    setNewPriority,
    setNewTaskType,
    newChecklistItems,
    latestDraftEvaluation,
    pendingSubtasks,
    addPendingSubtask,
    updatePendingSubtask,
    removePendingSubtask,
    newParentId,
    setNewParentId,
    newChecklistInherit,
    setNewChecklistInherit,
    appendNewChecklistCriterion,
    updateNewChecklistRow,
    removeNewChecklistRow,
    submitCreate,
    evaluateDraftBeforeCreate,
    startFreshDraft,
    resumeDraftByID,
    deleteDraftByID,
    createModalOpen,
    openCreateModal,
    closeCreateModal,
    editing,
    editTitle,
    setEditTitle,
    editPrompt,
    setEditPrompt,
    editPriority,
    editTaskType,
    setEditPriority,
    setEditTaskType,
    editStatus,
    setEditStatus,
    editChecklistInherit,
    setEditChecklistInherit,
    openEdit,
    closeEdit,
    submitEdit,
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
    taskListPage,
    setTaskListPage,
    resetTaskListPage,
    taskListPageSize: TASK_LIST_PAGE_SIZE,
    hasNextTaskPage,
    hasPrevTaskPage,
  };
}
