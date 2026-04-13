import { useQuery } from "@tanstack/react-query";
import { useCallback, useEffect, useState } from "react";
import { listTaskEvents } from "@/api";
import { TASK_EVENTS_PAGE_SIZE } from "../paging";
import { taskQueryKeys, type TaskEventsCursorKey } from "../queryKeys";

export function useTaskDetailEvents(taskId: string, enabled: boolean) {
  const [eventsCursor, setEventsCursor] = useState<TaskEventsCursorKey>({
    k: "head",
  });

  useEffect(() => {
    setEventsCursor({ k: "head" });
  }, [taskId]);

  const eventsQuery = useQuery({
    queryKey: taskQueryKeys.events(taskId, eventsCursor),
    queryFn: ({ signal }) => {
      const opts: {
        signal?: AbortSignal;
        limit: number;
        beforeSeq?: number;
        afterSeq?: number;
      } = { signal, limit: TASK_EVENTS_PAGE_SIZE };
      if (eventsCursor.k === "before") opts.beforeSeq = eventsCursor.seq;
      if (eventsCursor.k === "after") opts.afterSeq = eventsCursor.seq;
      return listTaskEvents(taskId, opts);
    },
    enabled: Boolean(taskId) && enabled,
  });

  const events = eventsQuery.data?.events ?? [];

  const onEventsPagerPrev = useCallback(() => {
    if (events.length === 0) return;
    const maxSeq = Math.max(...events.map((e) => e.seq));
    setEventsCursor({ k: "after", seq: maxSeq });
  }, [events]);

  const onEventsPagerNext = useCallback(() => {
    if (events.length === 0) return;
    const minSeq = Math.min(...events.map((e) => e.seq));
    setEventsCursor({ k: "before", seq: minSeq });
  }, [events]);

  const eventsTotal = eventsQuery.data?.total ?? 0;
  const timelineEvents = events;

  return {
    eventsQuery,
    timelineEvents,
    eventsTotal,
    onEventsPagerPrev,
    onEventsPagerNext,
  };
}
