/**
 * Wire types for `GET /system/health` (operator-facing observability
 * snapshot). Mirrors the Go envelope in
 * `internal/systemhealth/snapshot.go` field-for-field. Maps are
 * always present (zero-valued on a freshly-booted process) per the
 * docs/API-HTTP.md "System health" contract, so callers do not need
 * to branch on missing keys.
 */

export type SystemHealthBuild = {
  version: string;
  revision: string;
  go_version: string;
};

/** Status-class buckets aggregated from `taskapi_http_requests_total` `code`. */
export type SystemHealthRequestClass = "2xx" | "3xx" | "4xx" | "5xx" | "other";

/** p50/p95 are 0 when count is 0 (no requests recorded yet). */
export type SystemHealthHTTPDuration = {
  p50: number;
  p95: number;
  count: number;
};

export type SystemHealthHTTP = {
  in_flight: number;
  requests_total: number;
  /** Always seeded with all five class keys. */
  requests_by_class: Record<SystemHealthRequestClass, number>;
  duration_seconds: SystemHealthHTTPDuration;
};

export type SystemHealthSSE = {
  subscribers: number;
  dropped_frames_total: number;
};

export type SystemHealthDBPool = {
  max_open_connections: number;
  open_connections: number;
  in_use_connections: number;
  idle_connections: number;
  wait_count_total: number;
  wait_duration_seconds_total: number;
};

/** Terminal cycle statuses that the worker emits + a fallback bucket. */
export type SystemHealthAgentTerminalStatus =
  | "succeeded"
  | "failed"
  | "aborted"
  | "other";

export type SystemHealthAgent = {
  queue_depth: number;
  queue_capacity: number;
  runs_total: number;
  /** Always seeded with succeeded/failed/aborted; "other" appears only on worker bugs. */
  runs_by_terminal_status: Partial<
    Record<SystemHealthAgentTerminalStatus, number>
  >;
};

export type SystemHealthResponse = {
  build: SystemHealthBuild;
  uptime_seconds: number;
  /** RFC3339 wall clock at snapshot time. */
  now: string;
  http: SystemHealthHTTP;
  sse: SystemHealthSSE;
  db_pool: SystemHealthDBPool;
  agent: SystemHealthAgent;
};
