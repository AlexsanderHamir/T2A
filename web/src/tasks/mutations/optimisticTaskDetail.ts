import type { Priority, Status, Task, TaskListResponse } from "@/types";

export type TaskDetailPatchFields = {
  title: string;
  initial_prompt: string;
  status: Status;
  priority: Priority;
  project_id?: string | null;
  project_context_item_ids?: string[];
  tags?: string[];
  milestone?: string | null;
  cursor_model: string;
  pickup_not_before?: string | null;
};

export function mergePatchIntoTask(task: Task, patch: TaskDetailPatchFields): Task {
  return {
    ...task,
    title: patch.title,
    initial_prompt: patch.initial_prompt,
    status: patch.status,
    priority: patch.priority,
    project_id:
      patch.project_id === undefined ? task.project_id : patch.project_id ?? undefined,
    project_context_item_ids:
      patch.project_context_item_ids === undefined
        ? task.project_context_item_ids
        : patch.project_context_item_ids,
    tags: patch.tags === undefined ? task.tags : patch.tags,
    milestone: patch.milestone === undefined ? task.milestone : patch.milestone ?? undefined,
    cursor_model: patch.cursor_model,
    pickup_not_before:
      patch.pickup_not_before === undefined
        ? task.pickup_not_before
        : patch.pickup_not_before ?? undefined,
  };
}

export function patchTaskInList(
  list: TaskListResponse,
  taskId: string,
  patch: TaskDetailPatchFields,
): TaskListResponse | null {
  let changed = false;
  function visit(tasks: Task[]): Task[] {
    return tasks.map((t) => {
      if (t.id === taskId) {
        changed = true;
        return mergePatchIntoTask(t, patch);
      }
      return t;
    });
  }
  const next = visit(list.tasks);
  if (!changed) return null;
  return { ...list, tasks: next };
}
