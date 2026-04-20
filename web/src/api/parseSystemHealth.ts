import type {
  SystemHealthAgent,
  SystemHealthAgentTerminalStatus,
  SystemHealthBuild,
  SystemHealthDBPool,
  SystemHealthHTTP,
  SystemHealthHTTPDuration,
  SystemHealthRequestClass,
  SystemHealthResponse,
  SystemHealthSSE,
} from "@/types";

/**
 * Validates the `GET /system/health` JSON before the SystemHealth UI
 * relies on it. Mirrors `parseTaskStatsResponse` in spirit:
 *   • throws with a descriptive `Invalid API response: …` message,
 *   • seeds every documented enum key in nested maps (so the UI can
 *     index without `?? 0` everywhere),
 *   • uses `parseFiniteNumber` semantics inline (cannot import from
 *     parseTaskApi.ts without dragging the whole module).
 */

const REQUEST_CLASS_KEYS: readonly SystemHealthRequestClass[] = [
  "2xx",
  "3xx",
  "4xx",
  "5xx",
  "other",
];

const TERMINAL_STATUS_KEYS: readonly SystemHealthAgentTerminalStatus[] = [
  "succeeded",
  "failed",
  "aborted",
  "other",
];

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function parseFiniteNumber(v: unknown, field: string): number {
  if (typeof v !== "number" || !Number.isFinite(v)) {
    throw new Error(`Invalid API response: ${field} must be a number`);
  }
  return v;
}

function parseString(v: unknown, field: string): string {
  if (typeof v !== "string") {
    throw new Error(`Invalid API response: ${field} must be a string`);
  }
  return v;
}

export function parseSystemHealthResponse(value: unknown): SystemHealthResponse {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: system health payload must be an object");
  }
  return {
    build: parseBuild(value.build),
    uptime_seconds: parseFiniteNumber(value.uptime_seconds, "uptime_seconds"),
    now: parseString(value.now, "now"),
    http: parseHTTP(value.http),
    sse: parseSSE(value.sse),
    db_pool: parseDBPool(value.db_pool),
    agent: parseAgent(value.agent),
  };
}

function parseBuild(value: unknown): SystemHealthBuild {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: build must be an object");
  }
  return {
    version: parseString(value.version, "build.version"),
    revision: parseString(value.revision, "build.revision"),
    go_version: parseString(value.go_version, "build.go_version"),
  };
}

function parseHTTP(value: unknown): SystemHealthHTTP {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: http must be an object");
  }
  const requestsByClassRaw = value.requests_by_class;
  if (!isRecord(requestsByClassRaw)) {
    throw new Error("Invalid API response: http.requests_by_class must be an object");
  }
  // Seed every documented class so the UI can index unconditionally
  // even when one bucket is absent (the backend always seeds, but
  // older clients hitting newer servers should still degrade
  // gracefully rather than throw).
  const requests_by_class: Record<SystemHealthRequestClass, number> = {
    "2xx": 0,
    "3xx": 0,
    "4xx": 0,
    "5xx": 0,
    other: 0,
  };
  for (const [key, raw] of Object.entries(requestsByClassRaw)) {
    if (!REQUEST_CLASS_KEYS.includes(key as SystemHealthRequestClass)) {
      throw new Error(
        `Invalid API response: http.requests_by_class.${key} is not a known status class`,
      );
    }
    requests_by_class[key as SystemHealthRequestClass] = parseFiniteNumber(
      raw,
      `http.requests_by_class.${key}`,
    );
  }
  return {
    in_flight: parseFiniteNumber(value.in_flight, "http.in_flight"),
    requests_total: parseFiniteNumber(value.requests_total, "http.requests_total"),
    requests_by_class,
    duration_seconds: parseHTTPDuration(value.duration_seconds),
  };
}

function parseHTTPDuration(value: unknown): SystemHealthHTTPDuration {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: http.duration_seconds must be an object");
  }
  return {
    p50: parseFiniteNumber(value.p50, "http.duration_seconds.p50"),
    p95: parseFiniteNumber(value.p95, "http.duration_seconds.p95"),
    count: parseFiniteNumber(value.count, "http.duration_seconds.count"),
  };
}

function parseSSE(value: unknown): SystemHealthSSE {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: sse must be an object");
  }
  return {
    subscribers: parseFiniteNumber(value.subscribers, "sse.subscribers"),
    dropped_frames_total: parseFiniteNumber(
      value.dropped_frames_total,
      "sse.dropped_frames_total",
    ),
  };
}

function parseDBPool(value: unknown): SystemHealthDBPool {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: db_pool must be an object");
  }
  return {
    max_open_connections: parseFiniteNumber(
      value.max_open_connections,
      "db_pool.max_open_connections",
    ),
    open_connections: parseFiniteNumber(
      value.open_connections,
      "db_pool.open_connections",
    ),
    in_use_connections: parseFiniteNumber(
      value.in_use_connections,
      "db_pool.in_use_connections",
    ),
    idle_connections: parseFiniteNumber(
      value.idle_connections,
      "db_pool.idle_connections",
    ),
    wait_count_total: parseFiniteNumber(
      value.wait_count_total,
      "db_pool.wait_count_total",
    ),
    wait_duration_seconds_total: parseFiniteNumber(
      value.wait_duration_seconds_total,
      "db_pool.wait_duration_seconds_total",
    ),
  };
}

function parseAgent(value: unknown): SystemHealthAgent {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: agent must be an object");
  }
  const runsByStatusRaw = value.runs_by_terminal_status;
  if (!isRecord(runsByStatusRaw)) {
    throw new Error(
      "Invalid API response: agent.runs_by_terminal_status must be an object",
    );
  }
  const runs_by_terminal_status: SystemHealthAgent["runs_by_terminal_status"] = {
    succeeded: 0,
    failed: 0,
    aborted: 0,
  };
  for (const [key, raw] of Object.entries(runsByStatusRaw)) {
    if (!TERMINAL_STATUS_KEYS.includes(key as SystemHealthAgentTerminalStatus)) {
      throw new Error(
        `Invalid API response: agent.runs_by_terminal_status.${key} is not a known terminal status`,
      );
    }
    runs_by_terminal_status[key as SystemHealthAgentTerminalStatus] =
      parseFiniteNumber(raw, `agent.runs_by_terminal_status.${key}`);
  }
  return {
    queue_depth: parseFiniteNumber(value.queue_depth, "agent.queue_depth"),
    queue_capacity: parseFiniteNumber(value.queue_capacity, "agent.queue_capacity"),
    runs_total: parseFiniteNumber(value.runs_total, "agent.runs_total"),
    runs_by_terminal_status,
  };
}
