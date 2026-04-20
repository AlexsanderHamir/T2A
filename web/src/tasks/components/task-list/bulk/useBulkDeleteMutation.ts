import { useCallback, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { deleteTask as deleteTaskApi } from "@/api";
import { errorMessage } from "@/lib/errorMessage";
import {
  runWithConcurrency,
  type RunResult,
} from "@/lib/runWithConcurrency";
import { taskQueryKeys } from "../../../task-query";

/** Matches `BULK_SCHEDULE_CONCURRENCY` — same thundering-herd rationale. */
export const BULK_DELETE_CONCURRENCY = 5;

export type BulkDeleteFailure = {
  taskId: string;
  message: string;
};

export type BulkDeleteResult = {
  attempted: number;
  succeeded: number;
  failed: BulkDeleteFailure[];
};

/**
 * Fires N DELETE /tasks/{id} calls with a concurrency cap. Does not use
 * `useTaskDeleteFlow` optimistic cache surgery — on completion we
 * invalidate the task query namespace (same as bulk schedule PATCH).
 */
export function useBulkDeleteMutation() {
  const queryClient = useQueryClient();
  const [isPending, setPending] = useState(false);
  const [lastResult, setLastResult] = useState<BulkDeleteResult | null>(null);

  const reset = useCallback(() => {
    setLastResult(null);
  }, []);

  const run = useCallback(
    async (taskIds: ReadonlyArray<string>): Promise<BulkDeleteResult> => {
      if (taskIds.length === 0) {
        const empty: BulkDeleteResult = {
          attempted: 0,
          succeeded: 0,
          failed: [],
        };
        setLastResult(empty);
        return empty;
      }
      setPending(true);
      try {
        const calls = taskIds.map(
          (id) => () => deleteTaskApi(id),
        );
        const results: RunResult<void>[] = await runWithConcurrency(
          calls,
          BULK_DELETE_CONCURRENCY,
        );
        const failed: BulkDeleteFailure[] = [];
        let succeeded = 0;
        for (let i = 0; i < results.length; i++) {
          const r = results[i];
          if (r.ok) {
            succeeded++;
          } else {
            failed.push({
              taskId: taskIds[i],
              message: errorMessage(r.error, "Could not delete the task."),
            });
          }
        }
        const summary: BulkDeleteResult = {
          attempted: taskIds.length,
          succeeded,
          failed,
        };
        setLastResult(summary);
        await queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
        await queryClient.invalidateQueries({ queryKey: taskQueryKeys.stats() });
        return summary;
      } finally {
        setPending(false);
      }
    },
    [queryClient],
  );

  return { run, reset, isPending, lastResult } as const;
}
