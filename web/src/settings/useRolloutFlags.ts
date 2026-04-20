import { useQuery, useQueryClient } from "@tanstack/react-query";
import { fetchAppSettings, type AppSettings } from "@/api/settings";
import { settingsQueryKeys } from "@/tasks/task-query/queryKeys";

/**
 * Rollout feature flags introduced in
 * `.cursor/plans/production_realtime_smoothness_b17202b6.plan.md`.
 *
 * Read from the singleton `app_settings` row via the same query key
 * used by `useAppSettings`, so the gear icon badge, the Settings
 * page, and the per-mutation hooks all observe the same value and
 * re-render together when a settings_changed SSE event invalidates
 * the cache.
 *
 * The hook stays intentionally small: it doesn't return the mutation
 * handles (use `useAppSettings` for that), doesn't suspend (flag
 * consumers must handle "not loaded yet" gracefully, which they do
 * by falling back to the pessimistic path), and doesn't re-fetch —
 * piggy-backing on the existing query means zero extra HTTP traffic
 * on mount.
 */
export interface RolloutFlags {
  /**
   * When false, mutation hooks must bypass their onMutate/onError
   * optimistic code path and fall back to the legacy await-then-
   * render behavior. When the underlying query has not loaded yet
   * the hook returns `false` (pessimistic default) so the UI never
   * renders stale-optimistic state on first paint.
   */
  optimisticMutationsEnabled: boolean;
  /**
   * When false, the frontend should NOT set the Last-Event-ID header
   * on reconnect (no practical effect beyond saving a few bytes —
   * the server ignores it). The flag exists primarily so we can wire
   * the two rollout states through a single toggle in the SPA, even
   * though the server side is the load-bearing flip.
   */
  sseReplayEnabled: boolean;
  /**
   * True once the GET /settings response has populated the query
   * cache. Consumers that cannot tolerate a flicker may choose to
   * gate their "show optimistic UI" branch on this, but the default
   * policy is "pessimistic until loaded" regardless of this flag.
   */
  loaded: boolean;
}

export function useRolloutFlags(): RolloutFlags {
  const client = useQueryClient();
  // Subscribe (enabled:false) to the shared query so our hook reactively
  // re-renders when useAppSettings or an SSE invalidation updates it.
  // We never fire a network request from here; the primary consumer
  // (the Settings page / gear badge) owns the fetch.
  const query = useQuery<AppSettings>({
    queryKey: settingsQueryKeys.app(),
    queryFn: ({ signal }) => fetchAppSettings({ signal }),
    enabled: false,
    initialData: () => client.getQueryData<AppSettings>(settingsQueryKeys.app()),
  });
  const settings = query.data;
  if (!settings) {
    return {
      optimisticMutationsEnabled: false,
      sseReplayEnabled: false,
      loaded: false,
    };
  }
  return {
    optimisticMutationsEnabled: settings.optimistic_mutations_enabled,
    sseReplayEnabled: settings.sse_replay_enabled,
    loaded: true,
  };
}
