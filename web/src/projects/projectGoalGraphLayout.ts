import type { ProjectGoal } from "@/types";

/**
 * Assigns each goal to a layer index so dependencies appear to the left:
 * layer(g) = 1 + max(layer(dep)) for in-project deps, else 0.
 * Iterates until stable (≤ |goals| passes suffice for DAG depth).
 */
export function computeGoalLayers(goals: ProjectGoal[]): Map<string, number> {
  const idSet = new Set(goals.map((g) => g.id));
  const layer = new Map<string, number>();
  for (const g of goals) {
    layer.set(g.id, 0);
  }
  for (let iter = 0; iter < goals.length; iter++) {
    for (const g of goals) {
      let L = 0;
      for (const depId of g.depends_on_goal_ids) {
        if (!idSet.has(depId)) continue;
        L = Math.max(L, (layer.get(depId) ?? 0) + 1);
      }
      layer.set(g.id, L);
    }
  }
  return layer;
}

/** One column per layer; goals within a column sorted by title. */
export function goalsByLayerColumns(goals: ProjectGoal[]): ProjectGoal[][] {
  if (goals.length === 0) return [];
  const layer = computeGoalLayers(goals);
  const maxL = Math.max(0, ...layer.values());
  const cols: ProjectGoal[][] = Array.from({ length: maxL + 1 }, () => []);
  for (const g of goals) {
    cols[layer.get(g.id) ?? 0].push(g);
  }
  for (const c of cols) {
    c.sort((a, b) => a.title.localeCompare(b.title));
  }
  return cols;
}
