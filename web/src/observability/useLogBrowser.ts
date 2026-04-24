import { useInfiniteQuery, useQuery } from "@tanstack/react-query";
import { getLogEntries, listLogs } from "@/api/logs";
import type { LogEntriesResponse, LogEntryFilters, LogFileSummary } from "@/types";

export const LOG_LIST_QUERY_KEY = ["logs"] as const;
const LOG_ENTRY_PAGE_LIMIT = 150;

export type LogBrowserFilesState = {
  logs: LogFileSummary[];
  loading: boolean;
  unavailable: boolean;
};

export function useLogFiles(): LogBrowserFilesState {
  const query = useQuery({
    queryKey: LOG_LIST_QUERY_KEY,
    queryFn: async ({ signal }) => {
      try {
        return await listLogs({ signal });
      } catch {
        return null;
      }
    },
    refetchInterval: 15_000,
  });
  return {
    logs: query.data?.logs ?? [],
    loading: query.isPending,
    unavailable: query.data === null,
  };
}

export function useLogEntries(name: string | undefined, filters: LogEntryFilters) {
  return useInfiniteQuery<LogEntriesResponse | null>({
    queryKey: ["logs", name, filters],
    enabled: Boolean(name),
    initialPageParam: 0,
    queryFn: async ({ pageParam, signal }) => {
      if (!name) return null;
      try {
        return await getLogEntries(name, filters, {
          offset: Number(pageParam),
          limit: LOG_ENTRY_PAGE_LIMIT,
          signal,
        });
      } catch {
        return null;
      }
    },
    getNextPageParam: (lastPage) =>
      lastPage?.has_more ? lastPage.next_offset : undefined,
  });
}
