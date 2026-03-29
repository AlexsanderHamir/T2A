import { useQueryClient } from "@tanstack/react-query";
import { useCallback, useEffect, useRef, useState } from "react";
import { taskQueryKeys } from "../taskQueryKeys";

const SSE_INVALIDATE_MS = 400;

export function useTaskEventStream(): boolean {
  const queryClient = useQueryClient();
  const sseDebounceRef = useRef<ReturnType<typeof setTimeout> | undefined>();
  const [sseLive, setSseLive] = useState(false);

  const scheduleInvalidateFromStream = useCallback(() => {
    if (sseDebounceRef.current !== undefined) {
      clearTimeout(sseDebounceRef.current);
    }
    sseDebounceRef.current = setTimeout(() => {
      sseDebounceRef.current = undefined;
      void queryClient.invalidateQueries({ queryKey: taskQueryKeys.list() });
    }, SSE_INVALIDATE_MS);
  }, [queryClient]);

  useEffect(() => {
    const es = new EventSource("/events");
    es.onopen = () => setSseLive(true);
    es.onmessage = () => {
      scheduleInvalidateFromStream();
    };
    es.onerror = () => {
      setSseLive(false);
    };
    return () => {
      if (sseDebounceRef.current !== undefined) {
        clearTimeout(sseDebounceRef.current);
        sseDebounceRef.current = undefined;
      }
      es.close();
      setSseLive(false);
    };
  }, [scheduleInvalidateFromStream]);

  return sseLive;
}
