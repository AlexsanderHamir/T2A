import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import {
  addChecklistItem,
  createTask as apiCreate,
  deleteTask as apiDelete,
  listTasks,
  patchTask,
} from "../../api";
import type { PendingSubtaskDraft } from "../pendingSubtaskDraft";
import { flattenTaskTree, flattenTaskTreeRoots } from "../flattenTaskTree";
import { TASK_LIST_PAGE_SIZE } from "../paging";
import { taskQueryKeys } from "../queryKeys";
import {
  DEFAULT_NEW_TASK_STATUS,
  type Priority,
  type Status,
  type Task,
} from "@/types";
import { useHysteresisBoolean } from "@/lib/useHysteresisBoolean";
import { useTaskEventStream } from "./useTaskEventStream";

/** Background refetches (SSE invalidate, focus) are short; avoid UI flicker. */
const LIST_REFRESH_SHOW_MS = 380;
const LIST_REFRESH_HIDE_MS = 520;

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export function useTasksApp() {
  const queryClient = useQueryClient();
  const sseLive = useTaskEventStream();

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newPriority, setNewPriority] = useState<Priority>("medium");
  const [newChecklistDraft, setNewChecklistDraft] = useState("");
  const [newChecklistItems, setNewChecklistItems] = useState<string[]>([]);
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
  const [editStatus, setEditStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [editChecklistInherit, setEditChecklistInherit] = useState(false);

  /** In-app delete confirmation (avoids `window.confirm`, which breaks input focus in some browsers). */
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    title: string;
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
    setNewChecklistDraft("");
    setNewChecklistItems([]);
  }, [newChecklistInherit]);

  const resetNewTaskForm = useCallback(() => {
    setNewTitle("");
    setNewPrompt("");
    setNewPriority("medium");
    setNewChecklistDraft("");
    setNewChecklistItems([]);
    setPendingSubtasks([]);
    setNewParentId("");
    setNewChecklistInherit(false);
  }, []);

  const closeCreateModal = useCallback(() => {
    setCreateModalOpen(false);
    resetNewTaskForm();
  }, [resetNewTaskForm]);

  const openCreateModal = useCallback(() => {
    resetNewTaskForm();
    setCreateModalOpen(true);
  }, [resetNewTaskForm]);

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
      parent_id?: string;
      checklist_inherit: boolean;
      checklistItems: string[];
      pendingSubtasks: PendingSubtaskDraft[];
    }) => {
      const parentId = input.parent_id?.trim();
      const inherit =
        Boolean(parentId) && input.checklist_inherit === true;
      const task = await apiCreate({
        title: input.title,
        initial_prompt: input.initial_prompt,
        status: input.status,
        priority: input.priority,
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
      closeCreateModal();
    },
  });

  const patchMutation = useMutation({
    mutationFn: (args: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
      checklist_inherit: boolean;
    }) =>
      patchTask(args.id, {
        title: args.title,
        initial_prompt: args.initial_prompt,
        status: args.status,
        priority: args.priority,
        checklist_inherit: args.checklist_inherit,
      }),
    onSuccess: async () => {
      setEditing(null);
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiDelete(id),
    onSuccess: async (_, deletedId) => {
      setDeleteTarget(null);
      setEditing((prev) => (prev?.id === deletedId ? null : prev));
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
    },
  });

  const saving =
    createMutation.isPending ||
    patchMutation.isPending ||
    deleteMutation.isPending;

  const error = useMemo(() => {
    if (tasksQuery.isError) return errorMessage(tasksQuery.error);
    if (createMutation.isError) return errorMessage(createMutation.error);
    if (patchMutation.isError) return errorMessage(patchMutation.error);
    if (deleteMutation.isError) return errorMessage(deleteMutation.error);
    return editTitleRequiredError;
  }, [
    tasksQuery.isError,
    tasksQuery.error,
    createMutation.isError,
    createMutation.error,
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

  function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!newTitle.trim()) return;
    const parentId = newParentId.trim();
    createMutation.mutate({
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
      ...(parentId ? { parent_id: parentId } : {}),
      checklist_inherit: Boolean(parentId) && newChecklistInherit,
      checklistItems: newChecklistItems,
      pendingSubtasks,
    });
  }

  const addNewChecklistRow = useCallback(() => {
    const t = newChecklistDraft.trim();
    if (!t) return;
    setNewChecklistItems((prev) => [...prev, t]);
    setNewChecklistDraft("");
  }, [newChecklistDraft]);

  const removeNewChecklistRow = useCallback((index: number) => {
    setNewChecklistItems((prev) => prev.filter((_, i) => i !== index));
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
      checklist_inherit: editChecklistInherit,
    });
  }

  const requestDelete = useCallback((t: Task) => {
    setDeleteTarget({ id: t.id, title: t.title });
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  function confirmDelete() {
    if (!deleteTarget) return;
    deleteMutation.mutate(deleteTarget.id);
  }

  const createPending = createMutation.isPending;
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
    patchPending,
    deletePending,
    deleteMutation,
    error,
    sseLive,
    newTitle,
    setNewTitle,
    newPrompt,
    setNewPrompt,
    newPriority,
    setNewPriority,
    newChecklistDraft,
    setNewChecklistDraft,
    newChecklistItems,
    pendingSubtasks,
    addPendingSubtask,
    updatePendingSubtask,
    removePendingSubtask,
    newParentId,
    setNewParentId,
    newChecklistInherit,
    setNewChecklistInherit,
    addNewChecklistRow,
    removeNewChecklistRow,
    submitCreate,
    createModalOpen,
    openCreateModal,
    closeCreateModal,
    editing,
    editTitle,
    setEditTitle,
    editPrompt,
    setEditPrompt,
    editPriority,
    setEditPriority,
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
