import {
  MutationCache,
  QueryCache,
  QueryClient,
} from "@tanstack/react-query";

/** Dev-only: production builds omit cache `console.error` noise (UI still surfaces query errors). */
function logQueryError(scope: string, err: unknown): void {
  if (!import.meta.env.DEV) return;
  console.error(`[${scope}]`, err);
}

// Module-level flag that mirrors the `useTaskEventStream` connection
// state. We expose `set`/`get` rather than a React context so the
// QueryClient's `refetchOnWindowFocus` predicate — which runs outside
// the React render path — can read it without an extra subscription.
// Default `false` (treat SSE as not connected) so first-paint focus
// behaviour stays conservative until `useTaskEventStream` has had a
// chance to update it.
let sseLiveForQueries = false;

export function setSseLiveForQueries(connected: boolean): void {
  sseLiveForQueries = connected;
}

export function isSseLiveForQueries(): boolean {
  return sseLiveForQueries;
}

export function createAppQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (err) => logQueryError("tasks query", err),
    }),
    mutationCache: new MutationCache({
      onError: (err) => logQueryError("tasks mutation", err),
    }),
    defaultOptions: {
      queries: {
        staleTime: 15_000,
        gcTime: 5 * 60_000,
        // While the SSE stream is connected, the cache is already
        // being kept fresh by realtime events. Refetching on window
        // focus would just stampede the same endpoints we are already
        // observing through `/events`. When SSE is disconnected we
        // fall back to focus-based revalidation as a recovery hint.
        refetchOnWindowFocus: () => !isSseLiveForQueries(),
        retry: 1,
      },
      mutations: {
        retry: 0,
      },
    },
  });
}
