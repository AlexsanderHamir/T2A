import { useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useMemo, useState } from "react";
import { getTaskStats, listTasks } from "../../api";
import { TASK_TIMINGS } from "@/constants/tasks";
import { useHysteresisBoolean } from "@/lib/useHysteresisBoolean";
import { TASK_LIST_PAGE_SIZE } from "../task-paging";
import { taskQueryKeys } from "../task-query";
import { flattenTaskTreeRoots } from "../task-tree";

/** Background refetches (SSE invalidate, focus) are short; avoid UI flicker. */
const LIST_REFRESH_SHOW_MS = TASK_TIMINGS.listRefreshShowMs;
const LIST_REFRESH_HIDE_MS = TASK_TIMINGS.listRefreshHideMs;

/**
 * Task list + home-page stats queries and pagination for `useTasksApp`.
 * Split out to keep `useTasksApp.ts` within reviewable size (CODE_STANDARDS).
 */
export function useTasksAppListQuery() {
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

  const taskStatsQuery = useQuery({
    queryKey: ["task-stats"],
    queryFn: async ({ signal }) => {
      try {
        return await getTaskStats({ signal });
      } catch {
        return null;
      }
    },
  });

  const resetTaskListPage = useCallback(() => {
    setTaskListPage(0);
  }, []);

  const rootTaskTrees = useMemo(
    () => tasksQuery.data?.tasks ?? [],
    [tasksQuery.data?.tasks],
  );
  const tasks = useMemo(
    () => flattenTaskTreeRoots(rootTaskTrees),
    [rootTaskTrees],
  );

  const loading = tasksQuery.isPending;
  const rawListRefreshing =
    tasksQuery.isFetching && !tasksQuery.isPending;
  const listRefreshing = useHysteresisBoolean(
    rawListRefreshing,
    LIST_REFRESH_SHOW_MS,
    LIST_REFRESH_HIDE_MS,
  );

  useEffect(() => {
    if (!tasksQuery.isPending && rootTaskTrees.length === 0 && taskListPage > 0) {
      setTaskListPage(0);
    }
  }, [tasksQuery.isPending, rootTaskTrees.length, taskListPage]);

  const hasNextTaskPage = rootTaskTrees.length === TASK_LIST_PAGE_SIZE;
  const hasPrevTaskPage = taskListPage > 0;

  return {
    tasksQuery,
    taskStatsQuery,
    taskListPage,
    setTaskListPage,
    resetTaskListPage,
    tasks,
    rootTaskTrees,
    loading,
    listRefreshing,
    hasNextTaskPage,
    hasPrevTaskPage,
  };
}
