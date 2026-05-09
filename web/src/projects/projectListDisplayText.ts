/**
 * Display caps for goal/step rows and graph nodes. Full strings stay in the
 * model; truncation is presentation-only. Prefer `title` on the element when
 * `wasTruncated` is true so operators can hover the full value.
 */
export const PROJECT_LIST_TITLE_MAX = 56;
export const PROJECT_LIST_DESCRIPTION_MAX = 120;
export const PROJECT_LIST_DEPENDENCY_SUMMARY_MAX = 96;

export const PROJECT_GRAPH_TITLE_MAX = 48;
export const PROJECT_GRAPH_DESCRIPTION_MAX = 72;

function truncateEnd(s: string, max: number): string {
  if (max < 2) return "…";
  const t = s.trim();
  if (t.length <= max) return t;
  const chars = Array.from(t);
  if (chars.length <= max) return t;
  return chars.slice(0, max - 1).join("") + "…";
}

export function truncateListTitle(raw: string): string {
  return truncateEnd(raw, PROJECT_LIST_TITLE_MAX);
}

export function truncateListDescription(raw: string): string {
  return truncateEnd(raw, PROJECT_LIST_DESCRIPTION_MAX);
}

export function truncateListDependencySummary(raw: string): string {
  return truncateEnd(raw, PROJECT_LIST_DEPENDENCY_SUMMARY_MAX);
}

export function truncateGraphTitle(raw: string): string {
  return truncateEnd(raw, PROJECT_GRAPH_TITLE_MAX);
}

export function truncateGraphDescription(raw: string): string {
  return truncateEnd(raw, PROJECT_GRAPH_DESCRIPTION_MAX);
}
