import type { Priority, Status } from "@/types";
import type { TaskWithDepth } from "../../../flattenTaskTree";

export type TaskListClientStatusFilter = "all" | Status;
export type TaskListClientPriorityFilter = "all" | Priority;

/** Client-side filters for the task list (status, priority, title substring). */
export function filterTasksForListView(
  tasks: TaskWithDepth[],
  statusFilter: TaskListClientStatusFilter,
  priorityFilter: TaskListClientPriorityFilter,
  titleSearch: string,
): TaskWithDepth[] {
  const q = titleSearch.trim().toLowerCase();
  return tasks.filter((t) => {
    if (statusFilter !== "all" && t.status !== statusFilter) return false;
    if (priorityFilter !== "all" && t.priority !== priorityFilter)
      return false;
    if (q && !t.title.toLowerCase().includes(q)) return false;
    return true;
  });
}
