import {
  useCallback,
  useEffect,
  useState,
  type FormEvent,
} from "react";
import {
  createTask as apiCreate,
  deleteTask as apiDelete,
  listTasks,
  patchTask,
} from "../api";
import type { Priority, Status, Task } from "../types";

export function useTasksApp() {
  const [tasks, setTasks] = useState<Task[]>([]);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sseLive, setSseLive] = useState(false);

  const [newTitle, setNewTitle] = useState("");
  const [newPrompt, setNewPrompt] = useState("");
  const [newStatus, setNewStatus] = useState<Status>("ready");
  const [newPriority, setNewPriority] = useState<Priority>("medium");

  const [editing, setEditing] = useState<Task | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editStatus, setEditStatus] = useState<Status>("ready");
  const [editPriority, setEditPriority] = useState<Priority>("medium");

  /** In-app delete confirmation (avoids `window.confirm`, which breaks input focus in some browsers). */
  const [deleteTarget, setDeleteTarget] = useState<{
    id: string;
    title: string;
  } | null>(null);

  const refresh = useCallback(async () => {
    try {
      setError(null);
      const { tasks: next } = await listTasks();
      setTasks(next);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    const es = new EventSource("/events");
    es.onopen = () => setSseLive(true);
    es.onmessage = () => {
      void refresh();
    };
    es.onerror = () => {
      setSseLive(false);
    };
    return () => {
      es.close();
      setSseLive(false);
    };
  }, [refresh]);

  async function submitCreate(e: FormEvent) {
    e.preventDefault();
    if (!newTitle.trim()) return;
    setBusy(true);
    setError(null);
    try {
      await apiCreate({
        title: newTitle.trim(),
        initial_prompt: newPrompt,
        status: newStatus,
        priority: newPriority,
      });
      setNewTitle("");
      setNewPrompt("");
      setNewStatus("ready");
      setNewPriority("medium");
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  function openEdit(t: Task) {
    setEditing(t);
    setEditTitle(t.title);
    setEditPrompt(t.initial_prompt);
    setEditStatus(t.status);
    setEditPriority(t.priority);
  }

  function closeEdit() {
    setEditing(null);
  }

  async function submitEdit(e: FormEvent) {
    e.preventDefault();
    if (!editing) return;
    if (!editTitle.trim()) {
      setError("Title is required.");
      return;
    }
    setBusy(true);
    setError(null);
    try {
      await patchTask(editing.id, {
        title: editTitle.trim(),
        initial_prompt: editPrompt,
        status: editStatus,
        priority: editPriority,
      });
      setEditing(null);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
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
    setBusy(true);
    setError(null);
    try {
      await apiDelete(id);
      if (editing?.id === id) setEditing(null);
      await refresh();
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err));
    } finally {
      setBusy(false);
    }
  }

  return {
    tasks,
    loading,
    busy,
    error,
    sseLive,
    newTitle,
    setNewTitle,
    newPrompt,
    setNewPrompt,
    newStatus,
    setNewStatus,
    newPriority,
    setNewPriority,
    submitCreate,
    editing,
    editTitle,
    setEditTitle,
    editPrompt,
    setEditPrompt,
    editStatus,
    setEditStatus,
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
