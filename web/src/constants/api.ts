/**
 * API query/param constants mirrored from the Go handler/store layer.
 */

/** Matches `GET /tasks/cycle-failures` `sort` query (store cycle failure sorts). */
export const CYCLE_FAILURE_SORTS = [
  "at_desc",
  "at_asc",
  "reason_asc",
  "reason_desc",
] as const;

export type CycleFailureSort = (typeof CYCLE_FAILURE_SORTS)[number];
