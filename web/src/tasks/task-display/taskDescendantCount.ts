import type { Task } from "@/types/task";

/**
 * Total descendant count for a task tree (children + grandchildren …).
 * Excludes the root itself, mirrors the BFS contract documented for
 * `DELETE /tasks/{id}` in docs/API-HTTP.md: the server-side cascade
 * removes one row per id returned here when this root is deleted.
 *
 * Returns 0 for leaf rows and for inputs whose `children` array is
 * absent / empty (matching the shape-stable `omitempty` rule in the
 * task tree JSON: leaf rows omit `children` entirely, never serialize
 * `"children":[]`). Pure function — safe to call inside React render.
 */
export function taskDescendantCount(task: Pick<Task, "children">): number {
  const children = task.children;
  if (!children || children.length === 0) return 0;
  let total = 0;
  for (const child of children) {
    total += 1 + taskDescendantCount(child);
  }
  return total;
}
