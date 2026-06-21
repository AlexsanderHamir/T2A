import type { Task } from "@/types/task";

/** Newest-first by `created_at`, then id descending as a stable tie-break. */
export function sortTasksByCreatedDesc<T extends Task>(tasks: T[]): T[] {
  return [...tasks].sort((a, b) => {
    const aMs = a.created_at ? Date.parse(a.created_at) : 0;
    const bMs = b.created_at ? Date.parse(b.created_at) : 0;
    if (bMs !== aMs) return bMs - aMs;
    return b.id.localeCompare(a.id);
  });
}
