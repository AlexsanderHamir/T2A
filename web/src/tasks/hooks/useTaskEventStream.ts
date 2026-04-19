import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { parseTaskChangeFrame, settingsQueryKeys, taskQueryKeys } from "../task-query";

/**
 * Coalesce window for trailing-debounced SSE invalidations. The agent
 * worker emits ~6 `task_cycle_changed` frames per task run (StartCycle
 * → diagnose start/complete → execute start/complete → terminate),
 * spaced ~1–1.5s apart in real workloads. A short 400ms debounce never
 * actually batched them — every frame fired its own flush, kicking off
 * a refetch storm (events list + checklist + task row + cycles) on the
 * open task detail page roughly every second.
 *
 * 900ms is wide enough to cluster typical worker bursts into a single
 * flush, narrow enough that the user still sees status flips inside the
 * "feels live" budget. `MAX_WAIT_MS` is the safety valve: under a
 * continuous stream of frames (concurrent tasks, fast runner) the
 * debounce would otherwise reset forever and the UI would freeze; the
 * cap forces a flush at least every 2.5s no matter how busy the stream.
 *
 * Tuned with the agent's emission cadence in mind, NOT browser frame
 * rate — bumping the window further only delays the *first* visible
 * change for an idle-then-active task without improving throughput.
 */
const SSE_INVALIDATE_WINDOW_MS = 900;
const SSE_INVALIDATE_MAX_WAIT_MS = 2500;

/** Wait this long after an error before showing disconnected (browser may reconnect). */
const SSE_DISCONNECT_UI_MS = 900;

/**
 * Pending invalidation slots collected between debounce ticks. `tasks`
 * invalidates the entire `["tasks","detail",id]` subtree (covers all child
 * queries: events, checklist, cycles, the task row itself).
 *
 * Cycle frames (`task_cycle_changed`) are the *only* SSE signal the agent
 * worker emits — `task_updated` is HTTP-handler-only — so the worker's
 * status flips (running → done), audit-event appends, and checklist
 * toggles all surface as cycle frames. Treating them as task-scoped
 * invalidations is what keeps the task detail page actually live; the
 * earlier "cycles only" optimisation left events / checklist / status
 * stale until the user manually refreshed the page.
 */
type PendingInvalidations = {
  tasks: Set<string>;
  cycles: Map<string, Set<string>>;
};

function emptyPending(): PendingInvalidations {
  return { tasks: new Set(), cycles: new Map() };
}

function clearPending(p: PendingInvalidations): void {
  p.tasks.clear();
  p.cycles.clear();
}

export function useTaskEventStream(): boolean {
  const queryClient = useQueryClient();
  const sseDebounceRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const disconnectUiRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const pendingRef = useRef<PendingInvalidations>(emptyPending());
  /**
   * Wall-clock timestamp when the *current* pending flush window opened.
   * Reset to null after each flush. Used to enforce
   * SSE_INVALIDATE_MAX_WAIT_MS so a continuous stream of frames cannot
   * reset the trailing debounce indefinitely.
   */
  const firstQueuedAtRef = useRef<number | null>(null);
  /** Cleared on effect cleanup so queued timer callbacks cannot run after unmount. */
  const streamEffectActiveRef = useRef(false);
  const [sseLive, setSseLive] = useState(false);

  const flushStreamInvalidation = useCallback(() => {
    firstQueuedAtRef.current = null;
    const taskIds = [...pendingRef.current.tasks];
    const cycleEntries = [...pendingRef.current.cycles.entries()];
    clearPending(pendingRef.current);
    if (taskIds.length === 0 && cycleEntries.length === 0) {
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      return;
    }
    if (taskIds.length > 0) {
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
      for (const id of taskIds) {
        void queryClient.invalidateQueries({
          queryKey: taskQueryKeys.detail(id),
        });
      }
    }
    for (const [taskId] of cycleEntries) {
      if (taskIds.includes(taskId)) {
        // Already covered by the broad detail invalidation above.
        continue;
      }
      void queryClient.invalidateQueries({
        queryKey: taskQueryKeys.cycles(taskId),
      });
    }
  }, [queryClient]);

  const scheduleInvalidateFromStream = useCallback(
    (data: string) => {
      const frame = parseTaskChangeFrame(data);
      if (frame !== null) {
        if (frame.kind === "task") {
          pendingRef.current.tasks.add(frame.taskId);
        } else if (frame.kind === "cycle") {
          // The agent worker only emits cycle frames (it never calls
          // notifyChange / task_updated), so we must treat cycle frames
          // as broad task-scoped invalidations or the open task detail
          // page never sees worker-driven status flips, audit events, or
          // checklist toggles. The cycleId is still bucketed under
          // `cycles` for tests / analytics, but flushStreamInvalidation
          // de-duplicates against the broader `tasks` set so we don't
          // double-invalidate the cycles subtree.
          pendingRef.current.tasks.add(frame.taskId);
          let bucket = pendingRef.current.cycles.get(frame.taskId);
          if (bucket === undefined) {
            bucket = new Set();
            pendingRef.current.cycles.set(frame.taskId, bucket);
          }
          bucket.add(frame.cycleId);
        } else if (frame.kind === "settings" || frame.kind === "agent_run_cancelled") {
          // Settings updates and operator-initiated cancels are rare
          // and don't touch task data; refetch the settings cache
          // directly without joining the debounce batch (the SPA
          // Settings page should reflect the change instantly).
          void queryClient.invalidateQueries({
            queryKey: settingsQueryKeys.app(),
          });
        }
      }
      // Trailing debounce that respects a hard maxWait ceiling. New
      // frames push the flush back by SSE_INVALIDATE_WINDOW_MS, but we
      // also remember when the *first* pending frame landed so a
      // continuous stream cannot delay the flush past
      // SSE_INVALIDATE_MAX_WAIT_MS. Without the cap the debounce would
      // reset forever during back-to-back agent activity and the open
      // task page would freeze on stale data — exactly the smoothness
      // bug we are fixing here. Date.now() is intentional (not
      // performance.now()) because vitest fake timers mock the wall
      // clock and the existing test suite advances timers in ms ticks.
      const now = Date.now();
      if (firstQueuedAtRef.current === null) {
        firstQueuedAtRef.current = now;
      }
      const elapsedSinceFirst = now - firstQueuedAtRef.current;
      const remainingBudget = SSE_INVALIDATE_MAX_WAIT_MS - elapsedSinceFirst;
      const delay = Math.max(0, Math.min(SSE_INVALIDATE_WINDOW_MS, remainingBudget));
      if (sseDebounceRef.current !== undefined) {
        clearTimeout(sseDebounceRef.current);
      }
      sseDebounceRef.current = setTimeout(() => {
        sseDebounceRef.current = undefined;
        if (!streamEffectActiveRef.current) {
          return;
        }
        flushStreamInvalidation();
      }, delay);
    },
    [flushStreamInvalidation, queryClient],
  );

  useEffect(() => {
    streamEffectActiveRef.current = true;
    const es = new EventSource("/events");
    const clearDisconnectUi = () => {
      if (disconnectUiRef.current !== undefined) {
        clearTimeout(disconnectUiRef.current);
        disconnectUiRef.current = undefined;
      }
    };
    es.onopen = () => {
      clearDisconnectUi();
      if (!streamEffectActiveRef.current) {
        return;
      }
      setSseLive(true);
    };
    es.onmessage = (ev) => {
      scheduleInvalidateFromStream(String(ev.data ?? ""));
    };
    es.onerror = () => {
      clearDisconnectUi();
      disconnectUiRef.current = setTimeout(() => {
        disconnectUiRef.current = undefined;
        if (!streamEffectActiveRef.current) {
          return;
        }
        if (es.readyState !== EventSource.OPEN) {
          setSseLive(false);
        }
      }, SSE_DISCONNECT_UI_MS);
    };
    const pending = pendingRef.current;
    return () => {
      streamEffectActiveRef.current = false;
      clearDisconnectUi();
      if (sseDebounceRef.current !== undefined) {
        clearTimeout(sseDebounceRef.current);
        sseDebounceRef.current = undefined;
      }
      firstQueuedAtRef.current = null;
      clearPending(pending);
      es.close();
      setSseLive(false);
    };
  }, [scheduleInvalidateFromStream]);

  return sseLive;
}
