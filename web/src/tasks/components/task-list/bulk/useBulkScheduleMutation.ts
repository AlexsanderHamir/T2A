import { useCallback, useRef, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { patchTask as patchTaskApi } from "@/api";
import { errorMessage } from "@/lib/errorMessage";
import {
  runWithConcurrency,
  type RunResult,
} from "@/lib/runWithConcurrency";
import { taskQueryKeys } from "../../../task-query";

/**
 * Concurrency cap for bulk PATCHes. Five simultaneous requests is
 * enough to amortise round-trip latency on a normal connection
 * without thundering-herding the API for a 200-row selection. Per
 * the locked plan: "use pLimit-style local helper to avoid N=200
 * thundering herd". Tuneable later if real-world bulk sizes
 * routinely exceed 200; today the affordance for that scale is
 * non-existent (we only show the bar after explicit checkbox
 * selection).
 */
export const BULK_SCHEDULE_CONCURRENCY = 5;

export type BulkScheduleFailure = {
  taskId: string;
  message: string;
};

export type BulkScheduleResult = {
  attempted: number;
  succeeded: number;
  failed: BulkScheduleFailure[];
};

/**
 * useBulkScheduleMutation — fires N parallel PATCH /tasks/{id}
 * requests with `pickup_not_before` set to a single shared value,
 * with a concurrency cap of `BULK_SCHEDULE_CONCURRENCY`. Returns a
 * per-task aggregate result so callers can surface a single
 * combined error toast ("3 of 12 reschedules failed: …") instead
 * of N individual ones.
 *
 * The hook deliberately does NOT route through `useTaskPatchFlow`:
 * the latter requires a full TaskPatchInput shape (title, prompt,
 * status, priority, task_type, checklist_inherit) per task and
 * runs an optimistic-update path that mutates each row's cache
 * entry. For bulk operations with a single shared field
 * (`pickup_not_before`) the optimistic-per-row machinery is
 * overkill — we just invalidate the list once at the end and let
 * react-query refetch the page in a single round-trip. That
 * matches how `TaskDetailSchedule` (Stage 4) does it, keeping the
 * scheduling code path consistent.
 *
 * On any partial-failure we still invalidate, so the rows that
 * DID succeed render the new schedule and the operator sees the
 * same picture as the server.
 */
export function useBulkScheduleMutation() {
  const queryClient = useQueryClient();
  const [isPending, setPending] = useState(false);
  const [lastResult, setLastResult] = useState<BulkScheduleResult | null>(
    null,
  );
  /** Overlapping `run()` calls share one spinner; only clear when all finish. */
  const inFlightRef = useRef(0);

  const reset = useCallback(() => {
    setLastResult(null);
  }, []);

  const run = useCallback(
    async (
      taskIds: ReadonlyArray<string>,
      pickupNotBefore: string | null,
    ): Promise<BulkScheduleResult> => {
      if (taskIds.length === 0) {
        const empty: BulkScheduleResult = {
          attempted: 0,
          succeeded: 0,
          failed: [],
        };
        setLastResult(empty);
        return empty;
      }
      inFlightRef.current += 1;
      setPending(true);
      try {
        const calls = taskIds.map(
          (id) => () => patchTaskApi(id, { pickup_not_before: pickupNotBefore }),
        );
        const results: RunResult<unknown>[] = await runWithConcurrency(
          calls,
          BULK_SCHEDULE_CONCURRENCY,
        );
        const failed: BulkScheduleFailure[] = [];
        let succeeded = 0;
        for (let i = 0; i < results.length; i++) {
          const r = results[i];
          if (r.ok) {
            succeeded++;
          } else {
            failed.push({
              taskId: taskIds[i],
              message: errorMessage(r.error, "Could not update the schedule."),
            });
          }
        }
        const summary: BulkScheduleResult = {
          attempted: taskIds.length,
          succeeded,
          failed,
        };
        setLastResult(summary);
        // Always invalidate (even on partial failure) so successful
        // rows render the fresh schedule; failed rows refetch their
        // pre-attempt server-truth value, which is exactly what we
        // want the UI to reconcile to.
        await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
        await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
        return summary;
      } finally {
        inFlightRef.current -= 1;
        if (inFlightRef.current === 0) setPending(false);
      }
    },
    [queryClient],
  );

  return { run, reset, isPending, lastResult } as const;
}
