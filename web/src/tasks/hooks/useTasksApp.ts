import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState, type FormEvent } from "react";
import {
  createTask as apiCreate,
  deleteTask as apiDelete,
  listTasks,
  patchTask,
} from "../../api";
import { taskQueryKeys } from "../queryKeys";
import type { Priority, Status, Task } from "@/types";
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

  /** In-app delete confirmation (avoids `window.confirm`, which breaks input focus in some browsers). */
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    title: string;
  } | null>(null);

  /** Client-side validation (shown after server errors when applicable). */
  const [editTitleRequiredError, setEditTitleRequiredError] = useState<
    string | null
  >(null);

  const tasksQuery = useQuery({
    queryKey: taskQueryKeys.list(),
    queryFn: ({ signal }) => listTasks(200, 0, { signal }),
  });

  const tasks = tasksQuery.data?.tasks ?? [];
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
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.list() });
    },
  });

  const patchMutation = useMutation({
    mutationFn: (args: {
      id: string;
      title: string;
      initial_prompt: string;
      status: Status;
      priority: Priority;
    }) =>
      patchTask(args.id, {
        title: args.title,
        initial_prompt: args.initial_prompt,
        status: args.status,
        priority: args.priority,
      }),
    onSuccess: async () => {
      setEditing(null);
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.list() });
    },
  });

  const deleteMutation = useMutation({
    mutationFn: (id: string) => apiDelete(id),
    onSuccess: async (_, deletedId) => {
      setEditing((prev) => (prev?.id === deletedId ? null : prev));
      await queryClient.invalidateQueries({ queryKey: taskQueryKeys.list() });
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
      status: "ready",
      priority: newPriority,
    });
  }

  function openEdit(t: Task) {
    setEditing(t);
    setEditTitle(t.title);
    setEditPrompt(t.initial_prompt);
    setEditPriority(t.priority);
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
      status: editing.status,
      priority: editPriority,
    });
  }

  const requestDelete = useCallback((t: Task) => {
    setDeleteTarget({ id: t.id, title: t.title });
  }, []);

  const cancelDelete = useCallback(() => {
    setDeleteTarget(null);
  }, []);

  async function confirmDelete() {
    if (!deleteTarget) return;
    const { id } = deleteTarget;
    setDeleteTarget(null);
    deleteMutation.mutate(id);
  }

  return {
    tasks,
    loading,
    listRefreshing,
    saving,
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
    openEdit,
    closeEdit,
    submitEdit,
    deleteTarget,
    requestDelete,
    cancelDelete,
    confirmDelete,
  };
}
