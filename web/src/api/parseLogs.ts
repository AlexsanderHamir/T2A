import type {
  LogEntriesResponse,
  LogEntry,
  LogFileSummary,
  LogListResponse,
  LogRecord,
} from "@/types";

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

function parseString(v: unknown, field: string): string {
  if (typeof v !== "string") {
    throw new Error(`Invalid API response: ${field} must be a string`);
  }
  return v;
}

function parseNumber(v: unknown, field: string): number {
  if (typeof v !== "number" || !Number.isFinite(v)) {
    throw new Error(`Invalid API response: ${field} must be a number`);
  }
  return v;
}

function parseBoolean(v: unknown, field: string): boolean {
  if (typeof v !== "boolean") {
    throw new Error(`Invalid API response: ${field} must be a boolean`);
  }
  return v;
}

export function parseLogListResponse(value: unknown): LogListResponse {
  if (!isRecord(value) || !Array.isArray(value.logs)) {
    throw new Error("Invalid API response: logs payload must include logs[]");
  }
  return { logs: value.logs.map(parseLogFileSummary) };
}

function parseLogFileSummary(value: unknown): LogFileSummary {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: log summary must be an object");
  }
  return {
    name: parseString(value.name, "logs[].name"),
    size_bytes: parseNumber(value.size_bytes, "logs[].size_bytes"),
    modified_at: parseString(value.modified_at, "logs[].modified_at"),
  };
}

export function parseLogEntriesResponse(value: unknown): LogEntriesResponse {
  if (!isRecord(value) || !Array.isArray(value.entries)) {
    throw new Error("Invalid API response: log entries payload must include entries[]");
  }
  return {
    name: parseString(value.name, "name"),
    offset: parseNumber(value.offset, "offset"),
    limit: parseNumber(value.limit, "limit"),
    next_offset: parseNumber(value.next_offset, "next_offset"),
    has_more: parseBoolean(value.has_more, "has_more"),
    entries: value.entries.map(parseLogEntry),
  };
}

function parseLogEntry(value: unknown): LogEntry {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: log entry must be an object");
  }
  const entry: LogEntry = {
    line: parseNumber(value.line, "entries[].line"),
  };
  if (value.record !== undefined) {
    if (!isRecord(value.record)) {
      throw new Error("Invalid API response: entries[].record must be an object");
    }
    entry.record = value.record as LogRecord;
  }
  if (value.raw !== undefined) {
    entry.raw = parseString(value.raw, "entries[].raw");
  }
  if (value.parse_error !== undefined) {
    entry.parse_error = parseString(value.parse_error, "entries[].parse_error");
  }
  return entry;
}
