import type { QueryClient } from "@tanstack/react-query";
import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useMemo, useRef } from "react";
import { taskQueryKeys } from "@/tasks/task-query";
import { QUERY_POLICY } from "@/tasks/queryPolicy";
import { projectQueryKeys } from "@/projects/queryKeys";

/**
 * "Intent" event handlers fired on the kinds of events that signal a
 * navigation is imminent (pointer enter, focus). Spreading the result
 * onto an element wires both listeners without re-binding on every
 * render.
 */
export type IntentHandlers = {
  onPointerEnter: () => void;
  onFocus: () => void;
};

/**
 * Returns event handlers that invoke `fn` exactly once across the
 * component's mounted lifetime. Use for surfaces where a single
 * prefetch suffices (e.g. the header Settings link).
 *
 * The trigger is a no-op after the first fire — chunk imports and
 * `prefetchQuery` are already idempotent (Vite caches; React Query
 * dedups in-flight fetches) so this guard is purely an extra layer
 * to avoid running the closure twice.
 *
 * Failures inside `fn` are swallowed: prefetching is a hint, not a
 * critical path. The real navigation runs through the route resolver
 * and its own error boundary.
 */
export function usePrefetchOnIntent(fn: () => void): IntentHandlers {
  const firedRef = useRef(false);
  const fnRef = useRef(fn);
  fnRef.current = fn;

  const trigger = useCallback(() => {
    if (firedRef.current) {
      return;
    }
    firedRef.current = true;
    try {
      fnRef.current();
    } catch {
      /* prefetch is a perf hint; failures must not surface */
    }
  }, []);

  return useMemo(
    () => ({ onPointerEnter: trigger, onFocus: trigger }),
    [trigger],
  );
}

/**
 * Plain (non-hook) prefetch function suitable for list/grid rendering
 * loops where calling a hook per row would violate the rules of hooks
 * if the row count changes between renders. The function is safe to
 * fire repeatedly: the chunk import and `prefetchQuery` both dedup.
 */
export function prefetchTaskDetail(
  queryClient: QueryClient,
  taskId: string,
): void {
  if (!taskId) {
    return;
  }
  // Preload the lazy TaskDetailPage chunk. Idempotent: Vite caches the
  // module after the first import.
  void import("@/tasks/pages/TaskDetailPage");
  void queryClient.prefetchQuery({
    queryKey: taskQueryKeys.detail(taskId),
    queryFn: async ({ signal }) => {
      const { getTask } = await import("@/api/tasks");
      return getTask(taskId, { signal });
    },
    staleTime: QUERY_POLICY.prefetchStaleTimeMs,
  });
  void queryClient.prefetchQuery({
    queryKey: taskQueryKeys.checklist(taskId),
    queryFn: async ({ signal }) => {
      const { listChecklist } = await import("@/api/tasks");
      return listChecklist(taskId, { signal });
    },
    staleTime: QUERY_POLICY.prefetchStaleTimeMs,
  });
}

export function prefetchProjectDetail(
  queryClient: QueryClient,
  projectId: string,
): void {
  if (!projectId) {
    return;
  }
  void import("@/projects/ProjectDetailPage");
  void queryClient.prefetchQuery({
    queryKey: projectQueryKeys.detail(projectId),
    queryFn: async ({ signal }) => {
      const { getProject } = await import("@/api/projects");
      return getProject(projectId, { signal });
    },
    staleTime: QUERY_POLICY.prefetchStaleTimeMs,
  });
}

/**
 * Prefetches the SettingsPage chunk only — Settings reads the same
 * `settingsQueryKeys.app()` cache the bootstrap hook already seeded,
 * so no data prefetch is needed.
 */
export function useSettingsRoutePrefetch(): IntentHandlers {
  return usePrefetchOnIntent(() => {
    void import("@/settings/SettingsPage");
  });
}

/**
 * Returns a prefetcher closure bound to the current queryClient. Use
 * inside list-row map callbacks; the closure is safe to invoke
 * directly from `onPointerEnter` / `onFocus` handlers and the result
 * is stable across renders.
 */
export function useTaskDetailPrefetcher(): (taskId: string) => void {
  const queryClient = useQueryClient();
  return useCallback(
    (taskId: string) => prefetchTaskDetail(queryClient, taskId),
    [queryClient],
  );
}

export function useProjectDetailPrefetcher(): (projectId: string) => void {
  const queryClient = useQueryClient();
  return useCallback(
    (projectId: string) => prefetchProjectDetail(queryClient, projectId),
    [queryClient],
  );
}
