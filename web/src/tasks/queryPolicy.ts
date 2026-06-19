/** Central staleTime / gcTime tiers for TanStack Query. See ADR-0025. */
export const QUERY_POLICY = {
  /** Default for queries without an explicit tier. */
  defaultStaleTimeMs: 15_000,
  gcTimeMs: 5 * 60_000,
  /** Settings, project list, automations — bootstrap-seeded shell data. */
  shellStaleTimeMs: 5 * 60_000,
  /** Home list + stats while SSE may invalidate. */
  listStaleTimeMs: 60_000,
  /** Task detail, checklist, cycles, commits. */
  detailStaleTimeMs: 30_000,
  /** Hover prefetch for task detail navigation. */
  prefetchStaleTimeMs: 30_000,
  /** Session persist max age (Phase 4). */
  persistMaxAgeMs: 30 * 60_000,
} as const;
