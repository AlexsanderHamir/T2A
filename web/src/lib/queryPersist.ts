import type { Query } from "@tanstack/react-query";
import { projectQueryKeys } from "@/projects/queryKeys";
import { QUERY_POLICY } from "@/tasks/queryPolicy";
import { settingsQueryKeys, taskQueryKeys } from "@/tasks/task-query";

const LAST_DETAIL_ID_KEY = "hamix:last-detail-id";

export function isQueryPersistEnabled(): boolean {
  return import.meta.env.VITE_QUERY_PERSIST !== "0";
}

export function rememberPersistedDetailId(taskId: string): void {
  if (typeof sessionStorage === "undefined") return;
  sessionStorage.setItem(LAST_DETAIL_ID_KEY, taskId);
}

export function queryPersistMaxAgeMs(): number {
  return QUERY_POLICY.persistMaxAgeMs;
}

const PERSIST_STORAGE_KEY = "hamix:react-query";

export function queryPersistStorageKey(): string {
  return PERSIST_STORAGE_KEY;
}

/** Clears session-backed query cache after SSE resync (ADR-0025 Phase 4). */
export function bustQueryPersistCache(): void {
  if (typeof sessionStorage === "undefined") return;
  sessionStorage.removeItem(PERSIST_STORAGE_KEY);
}

export function shouldPersistQuery(query: Query): boolean {
  const key = query.queryKey;
  if (key[0] === settingsQueryKeys.all[0] && key[1] === "app") {
    return true;
  }
  if (key[0] === projectQueryKeys.all[0] && key[1] === "list") {
    return true;
  }
  if (
    key[0] === taskQueryKeys.all[0] &&
    key[1] === "detail" &&
    key.length === 3 &&
    typeof key[2] === "string"
  ) {
    if (typeof sessionStorage === "undefined") return false;
    const lastId = sessionStorage.getItem(LAST_DETAIL_ID_KEY);
    return lastId !== null && lastId === key[2];
  }
  return false;
}
