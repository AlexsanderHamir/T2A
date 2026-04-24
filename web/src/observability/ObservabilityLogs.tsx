import { FormEvent, useEffect, useMemo, useState } from "react";
import type { LogEntry, LogEntryFilters, LogFileSummary } from "@/types";
import { LogDetails, LogRows } from "./LogEntryTable";
import { useLogEntries, useLogFiles } from "./useLogBrowser";

const LEVELS = ["", "DEBUG", "INFO", "WARN", "ERROR"] as const;

type FilterForm = Required<Pick<LogEntryFilters, "level" | "operation" | "request_id" | "q">>;

const emptyFilters: FilterForm = {
  level: "",
  operation: "",
  request_id: "",
  q: "",
};

export function ObservabilityLogs() {
  const { logs, loading, unavailable } = useLogFiles();
  const [selectedName, setSelectedName] = useState<string>("");
  const [form, setForm] = useState<FilterForm>(emptyFilters);
  const [filters, setFilters] = useState<LogEntryFilters>({});
  const [selectedEntry, setSelectedEntry] = useState<LogEntry | null>(null);

  useEffect(() => {
    if (!selectedName && logs.length > 0) {
      setSelectedName(logs[0].name);
    }
  }, [logs, selectedName]);

  const selectedLog = logs.find((log) => log.name === selectedName);
  const entriesQuery = useLogEntries(selectedName || undefined, filters);
  const entries = useMemo(
    () => entriesQuery.data?.pages.flatMap((page) => page?.entries ?? []) ?? [],
    [entriesQuery.data],
  );

  function submitFilters(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    setSelectedEntry(null);
    setFilters(trimFilters(form));
  }

  function resetFilters() {
    setForm(emptyFilters);
    setFilters({});
    setSelectedEntry(null);
  }

  return (
    <section className="obs-logs" aria-label="Taskapi logs">
      <header className="obs-logs-head">
        <div>
          <h3 className="obs-logs-title">Logs</h3>
          <p className="obs-logs-subtitle">
            Browse local structured JSONL logs without opening files in the editor.
          </p>
        </div>
        <LogFileSelect
          logs={logs}
          selectedName={selectedName}
          loading={loading}
          onChange={(name) => {
            setSelectedName(name);
            setSelectedEntry(null);
          }}
        />
      </header>

      {unavailable ? (
        <p className="obs-logs-empty">Log browser unavailable.</p>
      ) : logs.length === 0 && !loading ? (
        <p className="obs-logs-empty">No taskapi log files found yet.</p>
      ) : (
        <>
          <LogFilters form={form} setForm={setForm} onSubmit={submitFilters} onReset={resetFilters} />
          <div className="obs-logs-meta">
            {selectedLog ? (
              <>
                <span>{selectedLog.name}</span>
                <span>{formatBytes(selectedLog.size_bytes)}</span>
                <time dateTime={selectedLog.modified_at}>
                  Updated {formatDate(selectedLog.modified_at)}
                </time>
              </>
            ) : (
              <span>{loading ? "Loading log files…" : "Select a log file"}</span>
            )}
          </div>
          <div className="obs-logs-layout">
            <LogRows
              entries={entries}
              loading={entriesQuery.isPending || entriesQuery.isFetching}
              selectedLine={selectedEntry?.line}
              onSelect={setSelectedEntry}
            />
            <LogDetails entry={selectedEntry} />
          </div>
          {entriesQuery.hasNextPage ? (
            <button
              type="button"
              className="obs-logs-load"
              onClick={() => void entriesQuery.fetchNextPage()}
              disabled={entriesQuery.isFetchingNextPage}
            >
              {entriesQuery.isFetchingNextPage ? "Loading…" : "Load more"}
            </button>
          ) : null}
        </>
      )}
    </section>
  );
}

function LogFileSelect({
  logs,
  selectedName,
  loading,
  onChange,
}: {
  logs: LogFileSummary[];
  selectedName: string;
  loading: boolean;
  onChange: (name: string) => void;
}) {
  return (
    <label className="obs-logs-file">
      <span>Log file</span>
      <select
        value={selectedName}
        onChange={(event) => onChange(event.target.value)}
        disabled={loading || logs.length === 0}
      >
        {logs.length === 0 ? <option value="">No files</option> : null}
        {logs.map((log) => (
          <option key={log.name} value={log.name}>
            {log.name}
          </option>
        ))}
      </select>
    </label>
  );
}

function LogFilters({
  form,
  setForm,
  onSubmit,
  onReset,
}: {
  form: FilterForm;
  setForm: (next: FilterForm) => void;
  onSubmit: (event: FormEvent<HTMLFormElement>) => void;
  onReset: () => void;
}) {
  return (
    <form className="obs-logs-filters" onSubmit={onSubmit}>
      <label>
        <span>Level</span>
        <select
          value={form.level}
          onChange={(event) => setForm({ ...form, level: event.target.value })}
        >
          {LEVELS.map((level) => (
            <option key={level || "all"} value={level}>
              {level || "All"}
            </option>
          ))}
        </select>
      </label>
      <label>
        <span>Operation</span>
        <input
          value={form.operation}
          onChange={(event) => setForm({ ...form, operation: event.target.value })}
          placeholder="handler.logs.entries"
        />
      </label>
      <label>
        <span>Request ID</span>
        <input
          value={form.request_id}
          onChange={(event) => setForm({ ...form, request_id: event.target.value })}
          placeholder="019…"
        />
      </label>
      <label>
        <span>Search</span>
        <input
          value={form.q}
          onChange={(event) => setForm({ ...form, q: event.target.value })}
          placeholder="slow query"
        />
      </label>
      <div className="obs-logs-filter-actions">
        <button type="submit">Apply</button>
        <button type="button" onClick={onReset}>
          Reset
        </button>
      </div>
    </form>
  );
}

function trimFilters(form: FilterForm): LogEntryFilters {
  return Object.fromEntries(
    Object.entries(form).map(([key, value]) => [key, value.trim()]).filter(([, value]) => value),
  ) as LogEntryFilters;
}

function formatDate(value: string): string {
  if (!value) return "—";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString();
}

function formatBytes(value: number): string {
  if (value < 1024) return `${value} B`;
  if (value < 1024 * 1024) return `${(value / 1024).toFixed(1)} KB`;
  return `${(value / (1024 * 1024)).toFixed(1)} MB`;
}
