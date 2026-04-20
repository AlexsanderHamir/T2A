import type {
  SystemHealthAgentTerminalStatus,
  SystemHealthRequestClass,
  SystemHealthResponse,
} from "@/types";

/**
 * Display order for HTTP request classes. Mirrors the order Prometheus
 * histograms surface in Grafana — successes first, then redirects,
 * client errors, server errors, and the catch-all bucket.
 */
export const REQUEST_CLASS_DISPLAY_ORDER: readonly SystemHealthRequestClass[] = [
  "2xx",
  "3xx",
  "4xx",
  "5xx",
  "other",
] as const;

/**
 * Display order for terminal agent run statuses. Aligns with the
 * status pill ordering on the task list so the Observability page
 * keeps the same color story.
 */
export const TERMINAL_STATUS_DISPLAY_ORDER: readonly SystemHealthAgentTerminalStatus[] = [
  "succeeded",
  "failed",
  "aborted",
  "other",
] as const;

export function requestClassLabel(c: SystemHealthRequestClass): string {
  return c === "other" ? "Other" : c.toUpperCase();
}

export function requestClassFillClass(c: SystemHealthRequestClass): string {
  switch (c) {
    case "2xx":
      return "cell-pill--status-done";
    case "3xx":
      return "cell-pill--status-running";
    case "4xx":
      return "cell-pill--status-blocked";
    case "5xx":
      return "cell-pill--status-failed";
    case "other":
    default:
      return "cell-pill--status-ready";
  }
}

export function terminalStatusLabel(s: SystemHealthAgentTerminalStatus): string {
  switch (s) {
    case "succeeded":
      return "Succeeded";
    case "failed":
      return "Failed";
    case "aborted":
      return "Aborted";
    case "other":
    default:
      return "Other";
  }
}

export function terminalStatusFillClass(s: SystemHealthAgentTerminalStatus): string {
  switch (s) {
    case "succeeded":
      return "cell-pill--status-done";
    case "failed":
      return "cell-pill--status-failed";
    case "aborted":
      return "cell-pill--status-blocked";
    case "other":
    default:
      return "cell-pill--status-ready";
  }
}

/**
 * Format a duration in seconds for compact KPI display. Picks the
 * smallest unit that yields a 1–3 digit integer-ish value:
 *   < 1 ms       → "0 ms"
 *   < 1 s        → "12 ms"
 *   < 60 s       → "12.3 s"
 *   < 60 min     → "12 min"
 *   < 24 h       → "12 h"
 *   ≥ 24 h       → "12 d"
 * The choice is deliberately operator-friendly (matches `journalctl`
 * and `kubectl get pods`) rather than scientifically precise.
 */
export function formatDurationSeconds(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds < 0.001) return "0 ms";
  if (seconds < 1) return `${Math.round(seconds * 1000)} ms`;
  if (seconds < 60) return `${formatNumber(seconds, 1)} s`;
  const minutes = seconds / 60;
  if (minutes < 60) return `${Math.round(minutes)} min`;
  const hours = minutes / 60;
  if (hours < 24) return `${Math.round(hours)} h`;
  return `${Math.round(hours / 24)} d`;
}

/**
 * Format a latency in seconds for the histogram quantile cells.
 * Always sub-second territory in healthy systems, so always render
 * milliseconds with one decimal.
 */
export function formatLatencySeconds(seconds: number): string {
  if (!Number.isFinite(seconds) || seconds < 0) return "—";
  if (seconds === 0) return "0 ms";
  const ms = seconds * 1000;
  if (ms < 1) return `${ms.toFixed(2)} ms`;
  if (ms < 10) return `${ms.toFixed(1)} ms`;
  if (ms < 1000) return `${Math.round(ms)} ms`;
  return `${formatNumber(seconds, 2)} s`;
}

/**
 * Locale-aware number formatter with optional fixed-decimal rendering.
 * Used for both counters (no decimals) and rates/seconds (1 decimal).
 */
export function formatNumber(value: number, decimals = 0): string {
  if (!Number.isFinite(value)) return "—";
  return value.toLocaleString(undefined, {
    minimumFractionDigits: decimals,
    maximumFractionDigits: decimals,
  });
}

/**
 * "12 / 64" style rendering used by the agent queue / DB pool gauges
 * where a current value sits inside a known capacity.
 */
export function formatRatio(current: number, capacity: number): string {
  if (capacity <= 0) return formatNumber(current);
  return `${formatNumber(current)} / ${formatNumber(capacity)}`;
}

export type SystemHealthSummary = {
  /** "OK" / "Degraded" / "Unknown" — drives the section accent. */
  level: "ok" | "degraded" | "unknown";
  /** Short human sentence shown next to the section title. */
  caption: string;
};

/**
 * Heuristic pane-level summary so the operator can see at a glance
 * whether anything is wrong without scanning every cell. Conservative
 * on purpose: anything beyond the listed signals stays "OK" so we
 * don't false-positive in a still-warming process.
 */
export function summarize(
  health: SystemHealthResponse | null | undefined,
  loading: boolean,
): SystemHealthSummary {
  if (!health) {
    return {
      level: "unknown",
      caption: loading ? "Loading system snapshot…" : "System snapshot unavailable.",
    };
  }
  const reasons: string[] = [];
  if (health.http.requests_by_class["5xx"] > 0) {
    reasons.push(
      `${formatNumber(health.http.requests_by_class["5xx"])} 5xx response${
        health.http.requests_by_class["5xx"] === 1 ? "" : "s"
      }`,
    );
  }
  if (health.sse.dropped_frames_total > 0) {
    reasons.push(
      `${formatNumber(health.sse.dropped_frames_total)} dropped SSE frame${
        health.sse.dropped_frames_total === 1 ? "" : "s"
      }`,
    );
  }
  const failedRuns = health.agent.runs_by_terminal_status.failed ?? 0;
  if (failedRuns > 0) {
    reasons.push(
      `${formatNumber(failedRuns)} failed agent run${failedRuns === 1 ? "" : "s"}`,
    );
  }
  if (
    health.db_pool.max_open_connections > 0 &&
    health.db_pool.in_use_connections >= health.db_pool.max_open_connections
  ) {
    reasons.push("DB pool saturated");
  }
  if (reasons.length === 0) {
    return {
      level: "ok",
      caption: `Build ${health.build.version} • up for ${formatDurationSeconds(
        health.uptime_seconds,
      )}`,
    };
  }
  return {
    level: "degraded",
    caption: `Attention: ${reasons.join(" · ")}`,
  };
}
