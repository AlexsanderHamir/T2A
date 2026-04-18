import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { getTaskCycle, listTaskCycles } from "@/api";
import type { TaskCycleDetail, TaskCyclesListResponse } from "@/types";
import { taskQueryKeys } from "../task-query";

/**
 * Fetches the cycle list for one task. Lives under the task detail subtree so
 * `task_updated` SSE invalidation sweeps it; granular `task_cycle_changed`
 * invalidation only touches `taskQueryKeys.cycles(taskId)` (see
 * `useTaskEventStream`).
 */
export function useTaskCycles(
  taskId: string,
  options?: { enabled?: boolean; limit?: number },
): UseQueryResult<TaskCyclesListResponse, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(taskId);
  return useQuery({
    queryKey: taskQueryKeys.cycles(taskId),
    queryFn: ({ signal }) => {
      const opts: { signal?: AbortSignal; limit?: number } = { signal };
      if (options?.limit !== undefined) opts.limit = options.limit;
      return listTaskCycles(taskId, opts);
    },
    enabled,
  });
}

/** Fetches one cycle (with phases) by id. Cached under `taskQueryKeys.cycle`. */
export function useTaskCycle(
  taskId: string,
  cycleId: string,
  options?: { enabled?: boolean },
): UseQueryResult<TaskCycleDetail, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(taskId) && Boolean(cycleId);
  return useQuery({
    queryKey: taskQueryKeys.cycle(taskId, cycleId),
    queryFn: ({ signal }) => getTaskCycle(taskId, cycleId, { signal }),
    enabled,
  });
}
