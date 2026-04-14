/** Keyset cursor for paged `GET /tasks/{id}/events` (stable when new events append). */
export type TaskEventsCursorKey =
  | { k: "head" }
  | { k: "before"; seq: number }
  | { k: "after"; seq: number };

export const taskQueryKeys = {
  all: ["tasks"] as const,
  /** Prefix for all task list queries (use with `invalidateQueries` partial match). */
  listRoot: () => [...taskQueryKeys.all, "list"] as const,
  list: (page: number) => [...taskQueryKeys.listRoot(), page] as const,
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
};
