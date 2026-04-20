import { useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchAppSettings, type AppSettings } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";

/**
 * Live behavior flags (optimistic mutations + SSE replay) are always
 * enabled in current builds; the hook keeps the same shape so mutation
 * hooks do not need churn. `loaded` still tracks whether GET /settings
 * has populated the cache (for cache-driven side effects).
 */
export interface RolloutFlags {
  optimisticMutationsEnabled: boolean;
  sseReplayEnabled: boolean;
  loaded: boolean;
}

export function useRolloutFlags(): RolloutFlags {
  const client = useQueryClient();
  const query = useQuery<AppSettings>({
    queryKey: settingsQueryKeys.app(),
    queryFn: ({ signal }) => fetchAppSettings({ signal }),
    enabled: false,
    initialData: () => client.getQueryData<AppSettings>(settingsQueryKeys.app()),
  });
  const settings = query.data;
  if (!settings) {
    return {
      optimisticMutationsEnabled: true,
      sseReplayEnabled: true,
      loaded: false,
    };
  }
  return {
    optimisticMutationsEnabled: true,
    sseReplayEnabled: true,
    loaded: true,
  };
}
