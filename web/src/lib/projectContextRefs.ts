import type { ProjectContextEdge, ProjectContextItem } from "@/types";

/**
 * Server cap from `pkgs/tasks/store/internal/tasks/project_context_selection.go`
 * (`maxSelectedProjectContextItems`). Mirrored here so the SPA can refuse to
 * add an item that would push past the limit instead of letting the API
 * reject the create/patch with a 400.
 */
export const MAX_SELECTED_PROJECT_CONTEXT_ITEMS = 20;

/**
 * Length of the human-facing short ID we render inside `#` mention chips and
 * the read-only REFERENCES block. Six characters is enough to distinguish a
 * few dozen siblings without bloating the chip and matches the convention
 * `#Decision title · a1b2c3`.
 */
export const PROJECT_CONTEXT_SHORT_ID_LENGTH = 6;

/**
 * Render a stable short identifier for a project context item. We strip
 * dashes (UUIDs) and underscores, then take the first N alphanumeric
 * characters lowercased. Falls back to the trimmed id when the cleaned
 * string is shorter than `PROJECT_CONTEXT_SHORT_ID_LENGTH` so callers
 * always get something printable.
 */
export function projectContextShortId(rawId: string): string {
  const trimmed = (rawId ?? "").trim();
  if (!trimmed) return "";
  const cleaned = trimmed.replace(/[^A-Za-z0-9]/g, "").toLowerCase();
  if (cleaned.length === 0) return trimmed.slice(0, PROJECT_CONTEXT_SHORT_ID_LENGTH);
  return cleaned.slice(0, PROJECT_CONTEXT_SHORT_ID_LENGTH);
}

/**
 * Outcome of a context-add gesture. `nodeOnly` adds just the chosen node;
 * `withChildren` follows outgoing project-context edges (same semantics as
 * `ProjectContextTreeView` — `source_context_id -> target_context_id`).
 */
export type ProjectContextAddMode = "nodeOnly" | "withChildren";

/**
 * Compute the set of context item IDs that should be selected when the user
 * picks `nodeId` with the given `mode`. For `nodeOnly`, returns `[nodeId]`.
 * For `withChildren`, includes `nodeId` plus all reachable descendants
 * following outgoing edges, with cycle protection.
 *
 * The returned list preserves the order: the chosen node first, then its
 * descendants in breadth-first order. Callers are expected to merge this
 * with the existing `selectedIds` via `mergeProjectContextSelection`.
 */
export function expandProjectContextSelection(
  nodeId: string,
  mode: ProjectContextAddMode,
  edges: readonly ProjectContextEdge[],
): string[] {
  const root = (nodeId ?? "").trim();
  if (!root) return [];
  if (mode === "nodeOnly") return [root];

  const childrenBySource = new Map<string, string[]>();
  for (const edge of edges) {
    const src = edge.source_context_id;
    const tgt = edge.target_context_id;
    if (!src || !tgt) continue;
    const list = childrenBySource.get(src) ?? [];
    list.push(tgt);
    childrenBySource.set(src, list);
  }

  const result: string[] = [];
  const seen = new Set<string>();
  const queue: string[] = [root];
  while (queue.length > 0) {
    const current = queue.shift()!;
    if (seen.has(current)) continue;
    seen.add(current);
    result.push(current);
    const children = childrenBySource.get(current) ?? [];
    for (const child of children) {
      if (!seen.has(child)) queue.push(child);
    }
  }
  return result;
}

/**
 * Merge `incoming` ids into `existing`, preserving order, deduping, and
 * stopping at `MAX_SELECTED_PROJECT_CONTEXT_ITEMS`. Returns the existing
 * array unchanged when nothing new would be appended (to keep React's
 * referential equality and skip needless re-renders).
 */
export function mergeProjectContextSelection(
  existing: readonly string[],
  incoming: readonly string[],
): string[] {
  if (incoming.length === 0) return existing.slice();
  const seen = new Set(existing);
  const merged = existing.slice();
  let changed = false;
  for (const id of incoming) {
    const trimmed = (id ?? "").trim();
    if (!trimmed) continue;
    if (seen.has(trimmed)) continue;
    if (merged.length >= MAX_SELECTED_PROJECT_CONTEXT_ITEMS) break;
    merged.push(trimmed);
    seen.add(trimmed);
    changed = true;
  }
  if (!changed) return existing.slice();
  return merged;
}

/**
 * Order the resolved `ProjectContextItem` records to match `selectedIds`,
 * dropping ids that no longer exist in the supplied items list. Used by the
 * REFERENCES block and the selected-summary panel so the visual order tracks
 * the operator's selection order.
 */
export function selectedProjectContextItems(
  items: readonly ProjectContextItem[],
  selectedIds: readonly string[],
): ProjectContextItem[] {
  if (selectedIds.length === 0 || items.length === 0) return [];
  const byId = new Map(items.map((item) => [item.id, item]));
  const out: ProjectContextItem[] = [];
  for (const id of selectedIds) {
    const item = byId.get(id);
    if (item) out.push(item);
  }
  return out;
}

/**
 * True when the reachable set from `nodeId` (following outgoing edges) is
 * strictly larger than the node itself. Used by the choice dialog to decide
 * whether to even offer the "Reference this node and its children" option.
 */
export function hasProjectContextChildren(
  nodeId: string,
  edges: readonly ProjectContextEdge[],
): boolean {
  const id = (nodeId ?? "").trim();
  if (!id) return false;
  return edges.some((edge) => edge.source_context_id === id);
}
