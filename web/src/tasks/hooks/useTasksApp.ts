import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import {
  createTask as apiCreate,
  deleteTask as apiDelete,
  listTasks,
  patchTask,
} from "../../api";
import { flattenTaskTree } from "../flattenTaskTree";
import { TASK_LIST_PAGE_SIZE } from "../paging";
import { taskQueryKeys } from "../queryKeys";
import {
  DEFAULT_NEW_TASK_STATUS,
  type Priority,
  type Status,
  type Task,
} from "@/types";
import { useTaskEventStream } from "./useTaskEventStream";

function errorMessage(e: unknown): string {
  return e instanceof Error ? e.message : String(e);
}

export function useTasksApp() {
  const queryClient = useQueryClient();
  const sseLive = useTaskEventStream();

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newPriority, setNewPriority] = useState<Priority>("medium");

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
    () => flattenTaskTree(rootTaskTrees),
    [rootTaskTrees],
  );
  const loading = tasksQuery.isPending;
  const listRefreshing =
    tasksQuery.isFetching && !tasksQuery.isPending;

  const createMutation = useMutation({
    mutationFn: (input: {
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
    }) => apiCreate(input),
    onSuccess: async () => {
      setNewTitle("");
      setNewPrompt("");
      setNewPriority("medium");
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
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
    createMutation.mutate({
      title: newTitle.trim(),
      initial_prompt: newPrompt,
      status: DEFAULT_NEW_TASK_STATUS,
      priority: newPriority,
    });
  }

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
    submitCreate,
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
