import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { setSseLiveForQueries } from "@/lib/queryClient";
import { connectTaskEventSource } from "./sseConnection";
import {
  dispatchTaskChangeFrame,
  flushProgressStreamInvalidation,
  flushStreamInvalidation,
} from "./sseCacheBridge";
import {
  clearPending,
  debounceDelayMs,
  emptyPending,
  PROGRESS_STREAM_INVALIDATE_MAX_WAIT_MS,
  PROGRESS_STREAM_INVALIDATE_WINDOW_MS,
  SSE_INVALIDATE_MAX_WAIT_MS,
  SSE_INVALIDATE_WINDOW_MS,
  type PendingInvalidations,
  type PendingProgressStreams,
} from "./sseInvalidationScheduler";

/**
 * Coalesce window for trailing-debounced SSE invalidations. The agent
 * worker emits ~4 `task_cycle_changed` frames per task run (StartCycle
 * → execute start/complete → terminate), spaced ~1–1.5s apart in real
 * workloads. A short 400ms debounce never actually batched them — every
 * frame fired its own flush, kicking off a refetch storm on the open
 * task detail page roughly every second.
 *
 * 900ms clusters typical worker bursts; `MAX_WAIT_MS` forces a flush at
 * least every 2.5s under continuous frames. See sseInvalidationScheduler
 * for constants; sseCacheBridge for enrichment vs hint-only flush logic.
 */
export function useTaskEventStream(): boolean {
  const queryClient = useQueryClient();
  const sseDebounceRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const progressStreamDebounceRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const pendingRef = useRef<PendingInvalidations>(emptyPending());
  const pendingProgressStreamsRef = useRef<PendingProgressStreams>(new Map());
  const firstQueuedAtRef = useRef<number | null>(null);
  const firstProgressStreamQueuedAtRef = useRef<number | null>(null);
  const streamEffectActiveRef = useRef(false);
  const [sseLive, setSseLive] = useState(false);

  const runFlushStreamInvalidation = useCallback(() => {
    firstQueuedAtRef.current = null;
    flushStreamInvalidation(queryClient, pendingRef.current);
  }, [queryClient]);

  const runFlushProgressStreamInvalidation = useCallback(() => {
    firstProgressStreamQueuedAtRef.current = null;
    flushProgressStreamInvalidation(queryClient, pendingProgressStreamsRef.current);
  }, [queryClient]);

  const scheduleProgressStreamInvalidation = useCallback(
    (taskId: string, cycleId: string) => {
      const streamKey = `${taskId}\u0000${cycleId}`;
      pendingProgressStreamsRef.current.set(streamKey, { taskId, cycleId });
      const now = Date.now();
      if (firstProgressStreamQueuedAtRef.current === null) {
        firstProgressStreamQueuedAtRef.current = now;
      }
      const delay = debounceDelayMs(
        now,
        firstProgressStreamQueuedAtRef.current,
        PROGRESS_STREAM_INVALIDATE_WINDOW_MS,
        PROGRESS_STREAM_INVALIDATE_MAX_WAIT_MS,
      );
      if (progressStreamDebounceRef.current !== undefined) {
        clearTimeout(progressStreamDebounceRef.current);
      }
      progressStreamDebounceRef.current = setTimeout(() => {
        progressStreamDebounceRef.current = undefined;
        if (!streamEffectActiveRef.current) {
          return;
        }
        runFlushProgressStreamInvalidation();
      }, delay);
    },
    [runFlushProgressStreamInvalidation],
  );

  const scheduleDebouncedFlush = useCallback(() => {
    const now = Date.now();
    if (firstQueuedAtRef.current === null) {
      firstQueuedAtRef.current = now;
    }
    const delay = debounceDelayMs(
      now,
      firstQueuedAtRef.current,
      SSE_INVALIDATE_WINDOW_MS,
      SSE_INVALIDATE_MAX_WAIT_MS,
    );
    if (sseDebounceRef.current !== undefined) {
      clearTimeout(sseDebounceRef.current);
    }
    sseDebounceRef.current = setTimeout(() => {
      sseDebounceRef.current = undefined;
      if (!streamEffectActiveRef.current) {
        return;
      }
      runFlushStreamInvalidation();
    }, delay);
  }, [runFlushStreamInvalidation]);

  const scheduleInvalidateFromStream = useCallback(
    (data: string) => {
      const result = dispatchTaskChangeFrame(
        data,
        queryClient,
        pendingRef.current,
        scheduleProgressStreamInvalidation,
      );
      if (result.kind === "immediate") {
        return;
      }
      if (result.kind === "resync") {
        if (sseDebounceRef.current !== undefined) {
          clearTimeout(sseDebounceRef.current);
          sseDebounceRef.current = undefined;
        }
        firstQueuedAtRef.current = null;
        return;
      }
      scheduleDebouncedFlush();
    },
    [queryClient, scheduleDebouncedFlush, scheduleProgressStreamInvalidation],
  );

  useEffect(() => {
    streamEffectActiveRef.current = true;
    const disconnect = connectTaskEventSource({
      isActive: () => streamEffectActiveRef.current,
      onMessage: scheduleInvalidateFromStream,
      onLiveChange: setSseLive,
    });
    const pending = pendingRef.current;
    const pendingProgressStreams = pendingProgressStreamsRef.current;
    return () => {
      streamEffectActiveRef.current = false;
      disconnect();
      if (sseDebounceRef.current !== undefined) {
        clearTimeout(sseDebounceRef.current);
        sseDebounceRef.current = undefined;
      }
      if (progressStreamDebounceRef.current !== undefined) {
        clearTimeout(progressStreamDebounceRef.current);
        progressStreamDebounceRef.current = undefined;
      }
      firstQueuedAtRef.current = null;
      firstProgressStreamQueuedAtRef.current = null;
      clearPending(pending);
      pendingProgressStreams.clear();
    };
  }, [scheduleInvalidateFromStream]);

  useEffect(() => {
    setSseLiveForQueries(sseLive);
    return () => {
      setSseLiveForQueries(false);
    };
  }, [sseLive]);

  return sseLive;
}
