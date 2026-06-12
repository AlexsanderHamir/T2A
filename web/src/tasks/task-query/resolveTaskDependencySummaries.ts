import type { QueryClient } from "@tanstack/react-query";
import type { Status, Task, TaskDependencyEdge, TaskListResponse } from "@/types";
import { taskQueryKeys } from "./queryKeys";

export type TaskDependencySummary = {
  id: string;
  title: string;
  status: Status;
};

function indexTask(task: Task, into: Map<string, TaskDependencySummary>): void {
  into.set(task.id, {
    id: task.id,
    title: task.title,
    status: task.status,
  });
  for (const child of task.children ?? []) {
    indexTask(child, into);
  }
}

function indexFromListResponse(data: TaskListResponse, into: Map<string, TaskDependencySummary>): void {
  for (const task of data.tasks) {
    indexTask(task, into);
  }
}

/** Resolves dependency edges to titles/statuses using cached task list/detail queries. */
export function resolveTaskDependencySummaries(
  queryClient: QueryClient,
  dependsOn: TaskDependencyEdge[],
): TaskDependencySummary[] {
  const index = new Map<string, TaskDependencySummary>();
  const queries = queryClient.getQueriesData<TaskListResponse | Task>({
    queryKey: taskQueryKeys.all,
  });
  for (const [, data] of queries) {
    if (!data) continue;
    if ("tasks" in data) {
      indexFromListResponse(data, index);
    } else {
      indexTask(data, index);
    }
  }
  return dependsOn.map((edge) => {
    const id = edge.task_id;
    const hit = index.get(id);
    if (hit) return hit;
    return { id, title: id, status: "ready" as Status };
  });
}
