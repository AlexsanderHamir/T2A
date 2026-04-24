export type LogFileSummary = {
  name: string;
  size_bytes: number;
  modified_at: string;
};

export type LogListResponse = {
  logs: LogFileSummary[];
};

export type LogRecord = Record<string, unknown>;

export type LogEntry = {
  line: number;
  record?: LogRecord;
  raw?: string;
  parse_error?: string;
};

export type LogEntriesResponse = {
  name: string;
  offset: number;
  limit: number;
  next_offset: number;
  has_more: boolean;
  entries: LogEntry[];
};

export type LogEntryFilters = {
  level?: string;
  operation?: string;
  request_id?: string;
  q?: string;
  from?: string;
  to?: string;
};
