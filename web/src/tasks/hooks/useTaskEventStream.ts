import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { taskQueryKeys } from "../queryKeys";
import { collectTaskIdFromSSEData } from "../sseInvalidate";

const SSE_INVALIDATE_MS = 400;

/** Wait this long after an error before showing disconnected (browser may reconnect). */
const SSE_DISCONNECT_UI_MS = 900;

export function useTaskEventStream(): boolean {
  const queryClient = useQueryClient();
  const sseDebounceRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const disconnectUiRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const pendingIdsRef = useRef<Set<string>>(new Set());
  /** Cleared on effect cleanup so queued timer callbacks cannot run after unmount. */
  const streamEffectActiveRef = useRef(false);
  const [sseLive, setSseLive] = useState(false);

  const flushStreamInvalidation = useCallback(() => {
    const ids = [...pendingIdsRef.current];
    pendingIdsRef.current.clear();
    if (ids.length === 0) {
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.all });
      return;
    }
    void queryClient.invalidateQueries({ queryKey: taskQueryKeys.listRoot() });
    for (const id of ids) {
      void queryClient.invalidateQueries({
        queryKey: [...taskQueryKeys.all, "detail", id],
      });
    }
  }, [queryClient]);

  const scheduleInvalidateFromStream = useCallback(
    (data: string) => {
      collectTaskIdFromSSEData(data, pendingIdsRef.current);
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
    [flushStreamInvalidation],
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
    const pendingIds = pendingIdsRef.current;
    return () => {
      streamEffectActiveRef.current = false;
      clearDisconnectUi();
      if (sseDebounceRef.current !== undefined) {
        clearTimeout(sseDebounceRef.current);
        sseDebounceRef.current = undefined;
      }
      pendingIds.clear();
      es.close();
      setSseLive(false);
    };
  }, [scheduleInvalidateFromStream]);

  return sseLive;
}
