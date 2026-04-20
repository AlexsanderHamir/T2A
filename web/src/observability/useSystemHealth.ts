import { useQuery } from "@tanstack/react-query";
import { getSystemHealth } from "@/api/system";
import type { SystemHealthResponse } from "@/types";

export const SYSTEM_HEALTH_QUERY_KEY = ["system-health"] as const;

/**
 * Cadence at which the operator-facing snapshot polls. Picked to keep
 * the in-flight / subscribers / queue numbers feeling "live" without
 * hammering the metrics aggregation path on every render. SSE does
 * not carry system-health events, so polling is the only refresh
 * lever — keep it long enough that several requests can land between
 * scrapes (10s = 1 datapoint per Prometheus tick, the same cadence
 * Grafana would use).
 */
export const SYSTEM_HEALTH_POLL_INTERVAL_MS = 10_000;

export type SystemHealthState = {
  /** `null` when the request errored after settling; `undefined` while pending. */
  health: SystemHealthResponse | null | undefined;
  loading: boolean;
};

/**
 * Read-side hook for `GET /system/health`. Mirrors the contract shape
 * of `useObservabilityStats`: pending → `undefined`, error → `null`,
 * success → response. The Observability page composes both hooks so
 * the SystemHealth pane and the task stats pane render without
 * coupling.
 */
export function useSystemHealth(): SystemHealthState {
  const query = useQuery({
    queryKey: SYSTEM_HEALTH_QUERY_KEY,
    queryFn: async ({ signal }) => {
      try {
        return await getSystemHealth({ signal });
      } catch {
        return null;
      }
    },
    refetchInterval: SYSTEM_HEALTH_POLL_INTERVAL_MS,
    refetchOnWindowFocus: true,
  });
  return { health: query.data, loading: query.isPending };
}
