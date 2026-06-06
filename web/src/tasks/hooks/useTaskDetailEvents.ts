import {
  useInfiniteQuery,
  type InfiniteData,
  type UseInfiniteQueryResult,
} from "@tanstack/react-query";
import { useCallback, useMemo } from "react";
import { listTaskEvents } from "@/api";
import type { TaskEvent, TaskEventsResponse } from "@/types";
import { TASK_EVENTS_PAGE_SIZE } from "../task-paging";
import { taskQueryKeys, type TaskEventsCursorKey } from "../task-query";

// Re-exported so the hook's keyset cursor type and the existing
// `taskQueryKeys.events()` shape stay in sync — the underlying
// page-param values use the same { head | before | after } variants.

/**
 * Bidirectional keyset cursor for `/tasks/{id}/events`. The head load
 * carries no cursor; subsequent fetches use `before_seq` to walk into
 * older events and `after_seq` to walk into newer ones. We expose the
 * same cursor shape that the query-keys module already encodes so the
 * page-param value used by `useInfiniteQuery` and the cache key tree
 * stay in lockstep.
 */
type CursorParam = TaskEventsCursorKey;

type EventsInfiniteData = InfiniteData<TaskEventsResponse, CursorParam>;

export type UseTaskDetailEventsResult = {
  eventsQuery: UseInfiniteQueryResult<EventsInfiniteData, Error>;
  /** Flat list of events across loaded pages, newest-first. */
  timelineEvents: TaskEvent[];
  /** Server-reported total for the task (taken from the head page). */
  eventsTotal: number;
  /** `true` when the most recent head page indicates an approval is pending. */
  approvalPending: boolean;
  /** Fetch the page just before the head (newer events). No-op if none. */
  onEventsPagerPrev: () => void;
  /** Fetch the page just after the tail (older events). No-op if none. */
  onEventsPagerNext: () => void;
};

/**
 * Task-detail events feed.
 *
 * Migrated to `useInfiniteQuery` so the pager preserves both
 * directions in a single cache entry rather than discarding pages on
 * each `setEventsCursor` flip. The previous useState/useQuery pair
 * replaced the cached page every time the user paged, which threw
 * away both the previously visible window and the React Query
 * "previousData" hint that smooths transitions.
 *
 * The page params are bidirectional keyset cursors derived from the
 * adjacent page's seq range (matching the server's `has_more_older` /
 * `has_more_newer` flags). React Query handles the actual cursor
 * walking through `getNextPageParam` / `getPreviousPageParam`; the
 * exposed `onEventsPagerPrev/Next` callbacks are thin wrappers that
 * trigger fetch in the correct direction.
 */
export function useTaskDetailEvents(
  taskId: string,
  enabled: boolean,
): UseTaskDetailEventsResult {
  // TanStack Query v5 does not infer `TPageParam` from
  // `initialPageParam` alone; the page-param type has to be supplied
  // explicitly so the `getNextPageParam` / `getPreviousPageParam`
  // return types and the data shape stay in sync with the cursor we
  // actually walk.
  const eventsQuery = useInfiniteQuery<
    TaskEventsResponse,
    Error,
    EventsInfiniteData,
    readonly [...typeof taskQueryKeys.all, "detail", string, "events", "infinite"],
    CursorParam
  >({
    queryKey: [
      ...taskQueryKeys.all,
      "detail",
      taskId,
      "events",
      "infinite",
    ] as const,
    initialPageParam: { k: "head" },
    queryFn: ({ pageParam, signal }) => {
      const opts: {
        signal?: AbortSignal;
        limit: number;
        beforeSeq?: number;
        afterSeq?: number;
      } = { signal, limit: TASK_EVENTS_PAGE_SIZE };
      if (pageParam.k === "before") opts.beforeSeq = pageParam.seq;
      if (pageParam.k === "after") opts.afterSeq = pageParam.seq;
      return listTaskEvents(taskId, opts);
    },
    // Walk older: server tells us via `has_more_older` whether the
    // last loaded page has predecessors. The seq cursor is the
    // minimum on that page so the next request returns strictly older
    // events.
    getNextPageParam: (last) => {
      if (!last.has_more_older || last.events.length === 0) return undefined;
      const minSeq = Math.min(...last.events.map((e) => e.seq));
      return { k: "before", seq: minSeq };
    },
    // Walk newer: same idea against `has_more_newer` and the maximum
    // seq on the first page.
    getPreviousPageParam: (first) => {
      if (!first.has_more_newer || first.events.length === 0) return undefined;
      const maxSeq = Math.max(...first.events.map((e) => e.seq));
      return { k: "after", seq: maxSeq };
    },
    enabled: Boolean(taskId) && enabled,
  });

  const pages = eventsQuery.data?.pages;

  const timelineEvents = useMemo<TaskEvent[]>(() => {
    if (!pages || pages.length === 0) return [];
    return pages.flatMap((page) => page.events);
  }, [pages]);

  // The head page is always at index 0 (initialPageParam = "head").
  // `total` and `approval_pending` are head-page properties; older
  // pages carry the same `total` but we read it from the head to be
  // explicit about the source of truth.
  const eventsTotal = pages?.[0]?.total ?? 0;
  const approvalPending = pages?.[0]?.approval_pending ?? false;

  const { fetchNextPage, fetchPreviousPage, hasNextPage, hasPreviousPage } =
    eventsQuery;

  const onEventsPagerNext = useCallback(() => {
    if (!hasNextPage) return;
    void fetchNextPage();
  }, [fetchNextPage, hasNextPage]);

  const onEventsPagerPrev = useCallback(() => {
    if (!hasPreviousPage) return;
    void fetchPreviousPage();
  }, [fetchPreviousPage, hasPreviousPage]);

  return {
    eventsQuery,
    timelineEvents,
    eventsTotal,
    approvalPending,
    onEventsPagerPrev,
    onEventsPagerNext,
  };
}
