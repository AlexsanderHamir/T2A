import {
  MutationCache,
  QueryCache,
  QueryClient,
} from "@tanstack/react-query";

function logQueryError(scope: string, err: unknown): void {
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
