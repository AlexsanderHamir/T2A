import { useQueryClient } from "@tanstack/react-query";
import { useEffect, useRef } from "react";
import { fetchBootstrap, type Bootstrap } from "@/api";
import { settingsQueryKeys, taskQueryKeys } from "@/tasks/task-query";
import { TASK_LIST_PAGE_SIZE } from "@/tasks/task-paging";
import { projectQueryKeys } from "@/projects/queryKeys";

/**
 * Bootstrap aggregate cache seeding hook.
 *
 * On App mount this fires a single request to `GET /v1/bootstrap` and,
 * on success, seeds every TanStack Query cache slot that the individual
 * page hooks (`useTasksApp`, `useTaskCreateFlow`, etc.) consume on cold
 * start. Those hooks remain unchanged: if the seed is already present
 * and within `staleTime`, React Query returns it synchronously and
 * skips the individual GET — the five cold-start round trips collapse
 * to one.
 *
 * Failure modes are deliberately silent:
 * - 404/405 (server too old): we return without seeding; per-page
 *   hooks fall through to their existing endpoint fan-out.
 * - Network / 5xx: same behaviour. The individual hooks will surface
 *   any user-visible errors via their own query state.
 *
 * The hook is fire-and-forget — it returns nothing. Components that
 * want to consume the data should do so through their existing query
 * hooks; the bootstrap path is invisible to them by design.
 */
export function useBootstrap(): void {
  const queryClient = useQueryClient();
  const hasRunRef = useRef(false);

  useEffect(() => {
    if (hasRunRef.current) {
      return;
    }
    hasRunRef.current = true;
    const controller = new AbortController();

    void (async () => {
      let payload: Bootstrap | null = null;
      try {
        payload = await fetchBootstrap({ signal: controller.signal });
      } catch (err) {
        // AbortError on unmount is the only expected failure here; any
        // other error means the per-endpoint hooks will surface it via
        // their own query state. Log in dev only — production silently
        // falls back to the per-page hooks.
        if (
          import.meta.env.DEV &&
          !(err instanceof DOMException && err.name === "AbortError")
        ) {
          console.warn("[bootstrap] aggregate fetch failed", err);
        }
        return;
      }
      if (payload === null || controller.signal.aborted) {
        return;
      }

      // setQueryData seeds the cache with a synthetic "freshly fetched"
      // entry. Subsequent useQuery hooks with the same key and within
      // staleTime read this directly without firing their own request.
      // The keys here MUST match the consumers' keys exactly:
      //   - useTasksApp:       settingsQueryKeys.app(),
      //                        taskQueryKeys.list({limit: TASK_LIST_PAGE_SIZE, offset: 0}),
      //                        taskQueryKeys.stats()
      //   - useTaskCreateFlow: taskQueryKeys.drafts()
      //   - useProjects:       projectQueryKeys.list(false, 100)
      queryClient.setQueryData(settingsQueryKeys.app(), payload.settings);
      queryClient.setQueryData(
        taskQueryKeys.list({ limit: TASK_LIST_PAGE_SIZE, offset: 0 }),
        payload.tasks,
      );
      queryClient.setQueryData(taskQueryKeys.stats(), payload.stats);
      queryClient.setQueryData(
        projectQueryKeys.list(false, 100),
        payload.projects,
      );
      queryClient.setQueryData(taskQueryKeys.drafts(), payload.drafts);
    })();

    return () => {
      controller.abort();
    };
  }, [queryClient]);
}
