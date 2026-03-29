import {
  MutationCache,
  QueryCache,
  QueryClient,
} from "@tanstack/react-query";

function logDevError(scope: string, err: unknown): void {
  if (import.meta.env.DEV) {
    console.error(`[${scope}]`, err);
  }
}

export function createAppQueryClient(): QueryClient {
  return new QueryClient({
    queryCache: new QueryCache({
      onError: (err) => logDevError("tasks query", err),
    }),
    mutationCache: new MutationCache({
      onError: (err) => logDevError("tasks mutation", err),
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
