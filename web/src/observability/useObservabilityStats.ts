import { useQuery } from "@tanstack/react-query";
import { getTaskStats } from "@/api/tasks";
import { taskQueryKeys } from "@/tasks/task-query";
import type { TaskStatsResponse } from "@/types/task";

/**
 * The shared cache key with `useTasksApp.taskStatsQuery`. Re-using it lets the
 * Observability page light up instantly when the user navigates from Home,
 * and lets `useTaskEventStream` invalidate one key for both consumers — see
 * web/src/tasks/hooks/useTaskEventStream.ts (`taskQueryKeys.stats()` invalidation).
 */
export const OBSERVABILITY_STATS_QUERY_KEY = taskQueryKeys.stats();

export type ObservabilityStatsState = {
  /** `null` when the request errored after settling; `undefined` while pending. */
  stats: TaskStatsResponse | null | undefined;
  loading: boolean;
};

/**
 * Read-side hook for `GET /tasks/stats` shared by Home KPI cards and the
 * Observability overview. Errors are swallowed to `null` so the caller can
 * render an "unavailable" state instead of throwing into the route error
 * boundary — matches the contract `useTasksApp.taskStatsQuery` expects.
 */
export function useObservabilityStats(): ObservabilityStatsState {
  const query = useQuery({
    queryKey: OBSERVABILITY_STATS_QUERY_KEY,
    queryFn: async ({ signal }) => {
      try {
        return await getTaskStats({ signal });
      } catch {
        return null;
      }
    },
  });
  return { stats: query.data, loading: query.isPending };
}
