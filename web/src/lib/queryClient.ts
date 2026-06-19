import {
  MutationCache,
  QueryCache,
  QueryClient,
} from "@tanstack/react-query";
import { isSseLiveForQueries } from "@/tasks/sync/connectionPolicy";

/** Dev-only: production builds omit cache `console.error` noise (UI still surfaces query errors). */
function logQueryError(scope: string, err: unknown): void {
  if (!import.meta.env.DEV) return;
  console.error(`[${scope}]`, err);
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
