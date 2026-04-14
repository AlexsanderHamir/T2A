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
        refetchOnWindowFocus: true,
        retry: 1,
      },
      mutations: {
        retry: 0,
      },
    },
  });
}
