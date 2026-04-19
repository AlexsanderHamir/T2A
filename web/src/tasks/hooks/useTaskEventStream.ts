import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { parseTaskChangeFrame, settingsQueryKeys, taskQueryKeys } from "../task-query";

const SSE_INVALIDATE_MS = 400;

/** Wait this long after an error before showing disconnected (browser may reconnect). */
const SSE_DISCONNECT_UI_MS = 900;

/**
 * Pending invalidation slots collected between debounce ticks. `tasks` invalidates
 * the entire `["tasks","detail",id]` subtree (covers all child queries).
 * `cycles` invalidates only the cycle subtree for a task — task detail,
 * checklist, events caches stay warm so cycle activity does not refetch the
 * world.
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
  /** Cleared on effect cleanup so queued timer callbacks cannot run after unmount. */
  const streamEffectActiveRef = useRef(false);
  const [sseLive, setSseLive] = useState(false);

  const flushStreamInvalidation = useCallback(() => {
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
      if (sseDebounceRef.current !== undefined) {
        clearTimeout(sseDebounceRef.current);
      }
      sseDebounceRef.current = setTimeout(() => {
        sseDebounceRef.current = undefined;
        if (!streamEffectActiveRef.current) {
          return;
        }
        flushStreamInvalidation();
      }, SSE_INVALIDATE_MS);
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
      clearPending(pending);
      es.close();
      setSseLive(false);
    };
  }, [scheduleInvalidateFromStream]);

  return sseLive;
}
