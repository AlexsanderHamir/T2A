import type { UseQueryResult } from "@tanstack/react-query";
import type { TaskEvent, TaskEventsResponse } from "@/types";
import { TaskPager } from "./TaskPager";
import { TaskUpdatesTimeline } from "./TaskUpdatesTimeline";

type Props = {
  taskId: string;
  eventsQuery: UseQueryResult<TaskEventsResponse>;
  timelineEvents: TaskEvent[];
  eventsTotal: number;
  onEventsPagerPrev: () => void;
  onEventsPagerNext: () => void;
};

export function TaskDetailUpdatesSection({
  taskId,
  eventsQuery,
  timelineEvents,
  eventsTotal,
  onEventsPagerPrev,
  onEventsPagerNext,
}: Props) {
  const data = eventsQuery.data;
  const pageCount = timelineEvents.length;

  const showPager =
    !eventsQuery.isPending &&
    !eventsQuery.isError &&
    eventsTotal > 0 &&
    Boolean(data?.has_more_newer || data?.has_more_older);

  const summary =
    data?.range_start !== undefined && data?.range_end !== undefined
      ? `${data.range_start}–${data.range_end} of ${eventsTotal}`
      : pageCount === 0
        ? "No rows on this page"
        : "—";

  return (
    <>
      <TaskUpdatesTimeline
        isPending={eventsQuery.isPending}
        isError={eventsQuery.isError}
        error={eventsQuery.error}
        timelineEvents={timelineEvents}
        isEmpty={
          !eventsQuery.isPending &&
          !eventsQuery.isError &&
          pageCount === 0 &&
          eventsTotal === 0
        }
        taskIdForLinks={taskId}
        onRetry={() => {
          void eventsQuery.refetch();
        }}
      />
      {showPager ? (
        <TaskPager
          navLabel="Update history pages"
          summary={summary}
          onPrev={onEventsPagerPrev}
          onNext={onEventsPagerNext}
          disablePrev={data?.has_more_newer !== true}
          disableNext={data?.has_more_older !== true}
        />
      ) : null}
    </>
  );
}
