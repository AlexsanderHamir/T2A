import type { Task } from "@/types";
import { taskDescendantCount } from "../task-display";

/**
 * Flat row carrying its display depth and the size of the descendant
 * subtree the original tree row carried. `descendantCount` is the BFS
 * total (children + grandchildren …) at the time of flattening; it is
 * preserved here even though `children` itself is stripped, so consumers
 * like the delete-confirm dialog can still warn the user that a single
 * `DELETE /tasks/{id}` will cascade (docs/API-HTTP.md cascade contract).
 *
 * `descendantCount` is optional so legacy fixtures and call-sites that
 * pre-date the cascade warning can construct a row by hand without it
 * (consumers default an absent value to 0 — same gracefully-degraded
 * behaviour as `subtaskCount` on the confirm dialog). The flatteners in
 * this file always populate it.
 */
export type TaskWithDepth = Task & { depth: number; descendantCount?: number };

/** Depth-first flattening of GET /tasks root trees (e.g. parent-task picker). */
export function flattenTaskTree(nodes: Task[], depth = 0): TaskWithDepth[] {
  const out: TaskWithDepth[] = [];
  for (const n of nodes) {
    const { children, ...rest } = n;
    out.push({ ...rest, depth, descendantCount: taskDescendantCount(n) });
    if (children?.length) {
      out.push(...flattenTaskTree(children, depth + 1));
    }
  }
  return out;
}

/** Top-level tasks only for the home list (no subtask rows). */
export function flattenTaskTreeRoots(nodes: Task[]): TaskWithDepth[] {
  return nodes.map((n) => {
    const { children, ...rest } = n;
    void children;
    return { ...rest, depth: 0, descendantCount: taskDescendantCount(n) };
  });
}
