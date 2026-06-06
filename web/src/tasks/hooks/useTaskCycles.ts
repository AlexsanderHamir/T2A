import {
  useInfiniteQuery,
  useQuery,
  type InfiniteData,
  type UseInfiniteQueryResult,
  type UseQueryResult,
} from "@tanstack/react-query";
import { useMemo } from "react";
import {
  getCycleVerdicts,
  getTaskCycle,
  listTaskCycleStreamEvents,
  listTaskCycles,
} from "@/api";
import type {
  CycleVerdictsResponse,
  TaskCycleDetail,
  TaskCycleStreamEvent,
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

type StreamCursor = { k: "head" } | { k: "after"; seq: number };
type StreamInfiniteData = InfiniteData<TaskCycleStreamResponse, StreamCursor>;

export type UseTaskCycleStreamResult = UseInfiniteQueryResult<
  StreamInfiniteData,
  Error
> & {
  /** Flat list of events across all loaded pages, in append order. */
  events: TaskCycleStreamEvent[];
  /** Wire-compatible head page (used by older consumers that read `.data?.events`). */
  headPage: TaskCycleStreamResponse | undefined;
};

/**
 * Cycle stream feed — append-only sequence of agent stream events.
 *
 * Migrated to `useInfiniteQuery` keyed by `after_seq` so the page-on
 * pattern matches the wire contract (`{ events, has_more,
 * next_after_seq }`). Previous implementation used a single
 * `useQuery` keyed only by `cycleId`, which meant every refetch
 * (window focus, SSE invalidation) re-downloaded the full window
 * starting from seq 0 — at 500 limit on a long-running cycle that
 * was the heaviest periodic request the SPA made.
 *
 * Consumers that only need the head window keep working: `headPage`
 * mirrors the old `.data` shape, and the convenience `events` getter
 * concatenates every loaded page in order.
 */
export function useTaskCycleStream(
  taskId: string,
  cycleId: string,
  options?: { enabled?: boolean; limit?: number },
): UseTaskCycleStreamResult {
  const enabled =
    (options?.enabled ?? true) && Boolean(taskId) && Boolean(cycleId);
  const limit = options?.limit;

  // TanStack Query v5 does not infer `TPageParam` from
  // `initialPageParam`; the page-param shape is supplied explicitly so
  // `getNextPageParam`'s return type lines up with the cursor we walk.
  const query = useInfiniteQuery<
    TaskCycleStreamResponse,
    Error,
    StreamInfiniteData,
    ReturnType<typeof taskQueryKeys.cycleStream>,
    StreamCursor
  >({
    queryKey: taskQueryKeys.cycleStream(taskId, cycleId),
    initialPageParam: { k: "head" },
    queryFn: ({ pageParam, signal }) => {
      const opts: { signal?: AbortSignal; limit?: number; afterSeq?: number } =
        { signal };
      if (limit !== undefined) opts.limit = limit;
      if (pageParam.k === "after") opts.afterSeq = pageParam.seq;
      return listTaskCycleStreamEvents(taskId, cycleId, opts);
    },
    // Cycle stream is forward-only (after_seq); there is no backward
    // walk because the head load already starts at the beginning of
    // the cycle. `getPreviousPageParam` therefore always returns
    // undefined, matching the wire contract.
    getNextPageParam: (last) => {
      if (!last.has_more) return undefined;
      const seq = last.next_after_seq;
      if (seq === undefined || seq <= 0) return undefined;
      return { k: "after", seq };
    },
    enabled,
  });

  const pages = query.data?.pages;
  const events = useMemo<TaskCycleStreamEvent[]>(() => {
    if (!pages || pages.length === 0) return [];
    return pages.flatMap((page) => page.events);
  }, [pages]);

  const headPage = pages?.[0];

  // We intentionally return a fresh object each render rather than
  // mutating React Query's internal result. The spread copies the
  // direct properties (data, isError, error, fetchNextPage, …) which
  // is all the public API; React Query does not stash hidden state on
  // the returned object.
  return { ...query, events, headPage };
}
