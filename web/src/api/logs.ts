import type { LogEntriesResponse, LogEntryFilters, LogListResponse } from "@/types";
import { parseLogEntriesResponse, parseLogListResponse } from "./parseLogs";
import { fetchWithTimeout, readError } from "./shared";

export async function listLogs(options?: {
  signal?: AbortSignal;
}): Promise<LogListResponse> {
  const res = await fetchWithTimeout("/logs", {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseLogListResponse(raw);
}

export async function getLogEntries(
  name: string,
  filters: LogEntryFilters,
  options?: { offset?: number; limit?: number; signal?: AbortSignal },
): Promise<LogEntriesResponse> {
  const params = new URLSearchParams();
  params.set("offset", String(options?.offset ?? 0));
  params.set("limit", String(options?.limit ?? 100));
  for (const [key, value] of Object.entries(filters)) {
    const trimmed = value?.trim();
    if (trimmed) {
      params.set(key, trimmed);
    }
  }
  const res = await fetchWithTimeout(`/logs/${encodeURIComponent(name)}?${params}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw new Error(await readError(res));
  const raw: unknown = await res.json();
  return parseLogEntriesResponse(raw);
}
