import { queryOptions, useQuery } from "@tanstack/react-query";
import { listCursorModels } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";
import { QUERY_POLICY } from "@/tasks/queryPolicy";

export function createModalCursorModelsQueryKey(
  runner: string,
  cursorBinKey: string,
) {
  return [
    ...settingsQueryKeys.all,
    "create-modal-cursor-models",
    runner,
    cursorBinKey,
  ] as const;
}

export function createModalCursorModelsQueryOptions(
  runner: string,
  cursorBinKey: string,
) {
  return queryOptions({
    queryKey: createModalCursorModelsQueryKey(runner, cursorBinKey),
    queryFn: ({ signal }) =>
      listCursorModels(
        {
          runner,
          binary_path: cursorBinKey || undefined,
        },
        { signal },
      ),
    enabled: runner === "cursor",
    staleTime: QUERY_POLICY.shellStaleTimeMs,
  });
}

export function useCreateModalCursorModels(runner: string, cursorBinKey: string) {
  return useQuery(createModalCursorModelsQueryOptions(runner, cursorBinKey));
}
