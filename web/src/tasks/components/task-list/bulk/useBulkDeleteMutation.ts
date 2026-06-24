import { useCallback } from "react";
import { deleteTask as deleteTaskApi } from "@/api";
import {
  useBulkTaskMutation,
  type BulkTaskFailure,
  type BulkTaskResult,
} from "./useBulkTaskMutation";

/** Matches `BULK_SCHEDULE_CONCURRENCY` — same thundering-herd rationale. */
export const BULK_DELETE_CONCURRENCY = 5;

export type BulkDeleteFailure = BulkTaskFailure;
export type BulkDeleteResult = BulkTaskResult;

/**
 * Fires N DELETE /tasks/{id} calls with a concurrency cap. Does not use
 * `useTaskDeleteFlow` optimistic cache surgery — on completion we
 * invalidate the task query namespace (same as bulk schedule PATCH).
 */
export function useBulkDeleteMutation() {
  const { run: runBulk, reset, isPending, lastResult } = useBulkTaskMutation({
    concurrency: BULK_DELETE_CONCURRENCY,
    failureMessage: "Could not delete the task.",
  });

  const run = useCallback(
    (taskIds: ReadonlyArray<string>) => runBulk(taskIds, (id) => deleteTaskApi(id)),
    [runBulk],
  );

  return { run, reset, isPending, lastResult } as const;
}
