import { useCallback, useEffect, useState, type FormEvent } from "react";
import {
  DEFAULT_NEW_TASK_STATUS,
  DEFAULT_NEW_TASK_TYPE,
  type Priority,
  type Status,
  type Task,
  type TaskType,
} from "@/types";
import { useTaskPatchFlow } from "./useTaskPatchFlow";

/**
 * Edit-task modal state and PATCH flow for `useTasksApp`.
 * Split out to keep `useTasksApp.ts` within reviewable size (CODE_STANDARDS).
 */
export function useTasksAppEditSheet() {
  const [editing, setEditing] = useState<Task | null>(null);
  const [editTitle, setEditTitle] = useState("");
  const [editPrompt, setEditPrompt] = useState("");
  const [editPriority, setEditPriority] = useState<Priority>("medium");
  const [editTaskType, setEditTaskType] = useState<TaskType>(DEFAULT_NEW_TASK_TYPE);
  const [editStatus, setEditStatus] = useState<Status>(DEFAULT_NEW_TASK_STATUS);
  const [editChecklistInherit, setEditChecklistInherit] = useState(false);

  /** Client-side validation (shown after server errors when applicable). */
  const [editTitleRequiredError, setEditTitleRequiredError] = useState<
    string | null
  >(null);

  const clearEditingIfTask = useCallback((taskId: string) => {
    setEditing((prev) => (prev?.id === taskId ? null : prev));
  }, []);

  const {
    patchTask: runPatch,
    patchPending,
    patchError,
    resetError: resetPatchError,
  } = useTaskPatchFlow({
    onPatched: (patchedId) => {
      clearEditingIfTask(patchedId);
    },
  });

  useEffect(() => {
    if (!editing) resetPatchError();
  }, [editing, resetPatchError]);

  useEffect(() => {
    if (editTitleRequiredError && editTitle.trim()) {
      setEditTitleRequiredError(null);
    }
  }, [editTitle, editTitleRequiredError]);

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
    runPatch({
      id: editing.id,
      title: editTitle.trim(),
      initial_prompt: editPrompt,
      status: editStatus,
      priority: editPriority,
      task_type: editTaskType,
      checklist_inherit: editChecklistInherit,
    });
  }

  return {
    editing,
    setEditing,
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
    editTitleRequiredError,
    openEdit,
    closeEdit,
    submitEdit,
    patchPending,
    patchError,
    resetPatchError,
    clearEditingIfTask,
  };
}
