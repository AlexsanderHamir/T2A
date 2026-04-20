/**
 * runWithConcurrency executes `tasks` in parallel with at most
 * `limit` in flight at any moment, returning a per-task result that
 * preserves input order and never throws.
 *
 * Each result is either `{ ok: true, value }` or `{ ok: false,
 * error }` so callers can aggregate failures into a single error
 * toast (e.g. "3 of 12 reschedules failed: ...") without needing
 * `Promise.allSettled`'s clunky discriminator dance. A failed task
 * does NOT stop the others — partial success is the common case
 * for bulk operations.
 *
 * Designed for the bulk-reschedule path in Stage 5: 5 PATCHes in
 * flight is enough to amortise the round-trip without thundering-
 * herding the API for a 200-row selection. The helper is
 * deliberately tiny (no external dep, no semaphore class) because
 * this is the only call site for the foreseeable future and the
 * common-case selection size is < 50.
 */
export type RunResult<T> =
  | { ok: true; value: T }
  | { ok: false; error: unknown };

export async function runWithConcurrency<T>(
  tasks: ReadonlyArray<() => Promise<T>>,
  limit: number,
): Promise<RunResult<T>[]> {
  if (tasks.length === 0) return [];
  const cap = Math.max(1, Math.min(limit, tasks.length));
  const results: RunResult<T>[] = new Array(tasks.length);
  let cursor = 0;

  const worker = async () => {
    while (true) {
      const idx = cursor++;
      if (idx >= tasks.length) return;
      try {
        const value = await tasks[idx]();
        results[idx] = { ok: true, value };
      } catch (error) {
        results[idx] = { ok: false, error };
      }
    }
  };

  const workers = Array.from({ length: cap }, () => worker());
  await Promise.all(workers);
  return results;
}
