/**
 * Shared three-state machine for headline counters.
 *
 * Lifted out of TaskHome so the new Observability page renders identical
 * skeleton / unavailable / ready behavior without reaching into the home
 * page module. The rule (kept verbatim from TaskHome): a numeric value
 * always wins; otherwise `loading` and "no stats settled yet" both map
 * to a skeleton, and a settled `null` payload maps to "—".
 */
export type KpiState =
  | { kind: "loading" }
  | { kind: "unavailable" }
  | { kind: "ready"; value: number };

export function kpiState(
  raw: number | undefined,
  loading: boolean,
  hasStats: boolean,
): KpiState {
  if (typeof raw === "number") return { kind: "ready", value: raw };
  if (loading || !hasStats) {
    return loading ? { kind: "loading" } : { kind: "unavailable" };
  }
  return { kind: "unavailable" };
}
