/** Keyset cursor for paged `GET /tasks/{id}/events` (stable when new events append). */
export type TaskEventsCursorKey =
  | { k: "head" }
  | { k: "before"; seq: number }
  | { k: "after"; seq: number };

/** Parameters that identify a single page of `GET /tasks`. */
export type TaskListParams = { limit: number; offset: number };

export const taskQueryKeys = {
  all: ["tasks"] as const,
  /** Prefix for all task list queries (use with `invalidateQueries` partial match). */
  listRoot: () => [...taskQueryKeys.all, "list"] as const,
  /**
   * A single page of `GET /tasks`. Keyed by `{ limit, offset }` so every
   * consumer — the home list (limit=20), the depends-on picker
   * (limit=200), the project tasks panel (limit=200) — shares the same
   * cache tree. Previously the picker / panel keyed off `listRoot()`
   * (the prefix itself), which created a parallel "all tasks" cache
   * entry duplicating storage, fetches, and SSE invalidation work.
   */
  list: (params: TaskListParams) =>
    [...taskQueryKeys.listRoot(), params] as const,
  detail: (id: string) => [...taskQueryKeys.all, "detail", id] as const,
  checklist: (id: string) =>
    [...taskQueryKeys.all, "detail", id, "checklist"] as const,
  events: (id: string, cursor: TaskEventsCursorKey) => {
    if (cursor.k === "head") {
      return [...taskQueryKeys.all, "detail", id, "events", "head"] as const;
    }
    if (cursor.k === "before") {
      return [
        ...taskQueryKeys.all,
        "detail",
        id,
        "events",
        "before",
        cursor.seq,
      ] as const;
    }
    return [
      ...taskQueryKeys.all,
      "detail",
      id,
      "events",
      "after",
      cursor.seq,
    ] as const;
  },
  eventDetail: (id: string, seq: number) =>
    [...taskQueryKeys.all, "detail", id, "event", seq] as const,
  /** Prefix for all cycle queries on a task; partial-match invalidation hits both list and per-cycle. */
  cycles: (id: string) =>
    [...taskQueryKeys.all, "detail", id, "cycles"] as const,
  cycle: (id: string, cycleId: string) =>
    [...taskQueryKeys.all, "detail", id, "cycles", cycleId] as const,
  cycleStream: (id: string, cycleId: string) =>
    [...taskQueryKeys.all, "detail", id, "cycles", cycleId, "stream"] as const,
  cycleVerdicts: (id: string, cycleId: string) =>
    [...taskQueryKeys.all, "detail", id, "cycles", cycleId, "verdicts"] as const,
  commits: (id: string) => [...taskQueryKeys.all, "detail", id, "commits"] as const,
  /**
   * GET /tasks/stats — shared by Home KPIs, Observability, and SSE invalidation
   * (lives outside `taskQueryKeys.all` prefix).
   */
  stats: () => ["task-stats"] as const,
  /**
   * GET /tasks/cycle-failures — prefix for all paginated failure-list queries.
   * Invalidated alongside task-stats when SSE reports task/cycle changes.
   */
  cycleFailuresRoot: () => [...taskQueryKeys.all, "cycle-failures"] as const,
  /** One page of the failures list (sort + offset identify the slice). */
  cycleFailures: (sort: string, offset: number) =>
    [...taskQueryKeys.cycleFailuresRoot(), sort, offset] as const,
  /** GET /task-drafts list and draft mutations invalidation. */
  drafts: () => ["task-drafts"] as const,
  /** GET /task-templates list; keyed by search query. */
  templates: (params?: { q?: string }) =>
    params?.q ? (["task-templates", { q: params.q }] as const) : (["task-templates"] as const),
};

export const settingsQueryKeys = {
  all: ["settings"] as const,
  app: () => [...settingsQueryKeys.all, "app"] as const,
};
