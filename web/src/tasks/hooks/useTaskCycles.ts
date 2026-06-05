import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import {
  getCycleVerdicts,
  getTaskCycle,
  listTaskCycleStreamEvents,
  listTaskCycles,
} from "@/api";
import type {
  CycleVerdictsResponse,
  TaskCycleDetail,
  TaskCycleStreamResponse,
  TaskCyclesListResponse,
} from "@/types";
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

/**
 * Fetches the per-criterion verdict envelope for one cycle. Pre-PR2
 * cycles return empty arrays inside the envelope (not 404), so the
 * hook stays in success state with `criteria_reports` / `verify_reports`
 * empty — UIs render that as "no verdicts captured".
 *
 * SSE invalidation: `task_cycle_changed` invalidates the
 * `taskQueryKeys.cycles(taskId)` prefix, which sweeps this query too.
 */
export function useTaskCycleVerdicts(
  taskId: string,
  cycleId: string,
  options?: { enabled?: boolean },
): UseQueryResult<CycleVerdictsResponse, Error> {
  const enabled =
    (options?.enabled ?? true) && Boolean(taskId) && Boolean(cycleId);
  return useQuery({
    queryKey: taskQueryKeys.cycleVerdicts(taskId, cycleId),
    queryFn: ({ signal }) => getCycleVerdicts(taskId, cycleId, { signal }),
    enabled,
  });
}

export function useTaskCycleStream(
  taskId: string,
  cycleId: string,
  options?: { enabled?: boolean; limit?: number },
): UseQueryResult<TaskCycleStreamResponse, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(taskId) && Boolean(cycleId);
  return useQuery({
    queryKey: taskQueryKeys.cycleStream(taskId, cycleId),
    queryFn: ({ signal }) => {
      const opts: { signal?: AbortSignal; limit?: number } = { signal };
      if (options?.limit !== undefined) opts.limit = options.limit;
      return listTaskCycleStreamEvents(taskId, cycleId, opts);
    },
    enabled,
  });
}
