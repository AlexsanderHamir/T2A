import type { LogEntry, LogRecord } from "@/types";

export function LogRows({
  entries,
  loading,
  selectedLine,
  onSelect,
}: {
  entries: LogEntry[];
  loading: boolean;
  selectedLine: number | undefined;
  onSelect: (entry: LogEntry) => void;
}) {
  if (entries.length === 0) {
    return (
      <div className="obs-logs-table obs-logs-empty-state">
        {loading ? "Loading log entries…" : "No log entries match the current filters."}
      </div>
    );
  }
  return (
    <div className="obs-logs-table" role="table" aria-label="Log entries">
      {entries.map((entry) => (
        <button
          key={entry.line}
          type="button"
          className={`obs-log-row ${selectedLine === entry.line ? "obs-log-row--selected" : ""}`}
          onClick={() => onSelect(entry)}
          role="row"
        >
          <span className="obs-log-cell obs-log-line">#{entry.line}</span>
          <span className={`obs-log-level obs-log-level--${logLevel(entry).toLowerCase()}`}>
            {logLevel(entry)}
          </span>
          <span className="obs-log-cell obs-log-time">
            {formatLogDate(recordString(entry.record, "time"))}
          </span>
          <span className="obs-log-cell obs-log-op">
            {recordString(entry.record, "operation") || "—"}
          </span>
          <span className="obs-log-cell obs-log-msg">
            {recordString(entry.record, "msg") || entry.parse_error || "—"}
          </span>
        </button>
      ))}
    </div>
  );
}

export function LogDetails({ entry }: { entry: LogEntry | null }) {
  if (!entry) {
    return (
      <aside className="obs-log-details" aria-label="Log entry details">
        <p>Select a row to inspect the full JSON payload.</p>
      </aside>
    );
  }
  return (
    <aside className="obs-log-details" aria-label="Log entry details">
      <h4>Line {entry.line}</h4>
      <dl>
        <div>
          <dt>Operation</dt>
          <dd>{recordString(entry.record, "operation") || "—"}</dd>
        </div>
        <div>
          <dt>Request ID</dt>
          <dd>{recordString(entry.record, "request_id") || "—"}</dd>
        </div>
      </dl>
      <pre>
        {JSON.stringify(entry.record ?? { raw: entry.raw, parse_error: entry.parse_error }, null, 2)}
      </pre>
    </aside>
  );
}

function recordString(record: LogRecord | undefined, key: string): string {
  const value = record?.[key];
  return typeof value === "string" ? value : "";
}

function logLevel(entry: LogEntry): string {
  return recordString(entry.record, "level") || (entry.parse_error ? "INVALID" : "INFO");
}

function formatLogDate(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}
