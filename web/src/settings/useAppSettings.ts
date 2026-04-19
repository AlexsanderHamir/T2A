import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  type AppSettings,
  type AppSettingsPatch,
  type CancelCurrentRunResult,
  type ProbeCursorResult,
  cancelCurrentRun,
  fetchAppSettings,
  patchAppSettings,
  probeCursor,
} from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";

/**
 * React Query hook for the singleton AppSettings row. The query
 * cache is shared across the SPA so the gear-icon badge, the
 * Settings page, and any future "current runner" indicator all
 * see the same value (and re-render together when an SSE
 * settings_changed frame triggers an invalidation).
 *
 * The hook intentionally exposes the raw mutation objects (not
 * thin onClick wrappers) so callers can drive their own loading,
 * error, and success copy without re-implementing react-query
 * state machines.
 */
export function useAppSettings() {
  const queryClient = useQueryClient();

  const settingsQuery = useQuery<AppSettings>({
    queryKey: settingsQueryKeys.app(),
    queryFn: ({ signal }) => fetchAppSettings({ signal }),
  });

  const patchMutation = useMutation<AppSettings, Error, AppSettingsPatch>({
    mutationFn: (patch) => patchAppSettings(patch),
    onSuccess: (next) => {
      queryClient.setQueryData<AppSettings>(settingsQueryKeys.app(), next);
    },
  });

  const probeMutation = useMutation<
    ProbeCursorResult,
    Error,
    { runner?: string; binary_path?: string }
  >({
    mutationFn: (body) => probeCursor(body),
  });

  const cancelMutation = useMutation<CancelCurrentRunResult, Error, void>({
    mutationFn: () => cancelCurrentRun(),
  });

  return {
    settings: settingsQuery.data,
    isLoading: settingsQuery.isLoading,
    error: settingsQuery.error,
    refetch: () => settingsQuery.refetch(),
    patch: patchMutation,
    probe: probeMutation,
    cancelRun: cancelMutation,
  };
}
