import type { Priority, Status } from "@/types";
import type { TaskWithDepth } from "../../../task-tree";

/**
 * The status filter accepts every real `Status`, plus "all" and a
 * synthetic "scheduled" bucket. The synthetic bucket is *not* a row
 * status — it's a virtual cross-section over `(status === "ready" &&
 * pickup_not_before > now)`, surfaced as a dedicated filter so an
 * operator can answer "show me everything queued for the future"
 * without scrolling the whole ready list. Per the locked decision
 * `scope=agent_only`, scheduled tasks are NEVER hidden from the
 * default list — this is a UX affordance only, never a data filter.
 */
export type TaskListClientStatusFilter = "all" | "scheduled" | Status;
export type TaskListClientPriorityFilter = "all" | Priority;

/** Client-side filters for the task list (status, priority, title substring). */
export function filterTasksForListView(
  tasks: TaskWithDepth[],
  statusFilter: TaskListClientStatusFilter,
  priorityFilter: TaskListClientPriorityFilter,
  titleSearch: string,
  /**
   * Override for `Date.now()` when evaluating the synthetic
   * `scheduled` bucket. Tests pass a fixed clock so the cutoff is
   * deterministic; production callers leave it undefined and we read
   * from `Date.now`.
   */
  nowMs?: number,
): TaskWithDepth[] {
  const q = titleSearch.trim().toLowerCase();
  const now = nowMs ?? Date.now();
  return tasks.filter((t) => {
    if (statusFilter === "scheduled") {
      if (t.status !== "ready") return false;
      if (!t.pickup_not_before) return false;
      const ts = Date.parse(t.pickup_not_before);
      if (Number.isNaN(ts)) return false;
      if (ts <= now) return false;
    } else if (statusFilter !== "all" && t.status !== statusFilter) {
      return false;
    }
    if (priorityFilter !== "all" && t.priority !== priorityFilter)
      return false;
    if (q && !t.title.toLowerCase().includes(q)) return false;
    return true;
  });
}
