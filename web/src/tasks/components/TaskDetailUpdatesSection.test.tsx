import type { UseQueryResult } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import type { TaskEvent, TaskEventsResponse } from "@/types";
import { ROUTER_FUTURE_FLAGS } from "../../lib/routerFutureFlags";
import { TaskDetailUpdatesSection } from "./TaskDetailUpdatesSection";

function asEventsQuery(
  partial: Partial<UseQueryResult<TaskEventsResponse>> &
    Pick<UseQueryResult<TaskEventsResponse>, "isPending" | "isError" | "error" | "data">,
): UseQueryResult<TaskEventsResponse> {
  return partial as UseQueryResult<TaskEventsResponse>;
}

const sampleEvent: TaskEvent = {
  seq: 1,
  at: "2026-01-01T00:00:00.000Z",
  type: "message_added",
  by: "agent",
  data: {},
};

describe("TaskDetailUpdatesSection", () => {
  it("shows timeline skeleton while events are loading", () => {
    render(
      <TaskDetailUpdatesSection
        taskId="t1"
        eventsQuery={asEventsQuery({
          isPending: true,
          isError: false,
          error: null,
          data: undefined,
        })}
        timelineEvents={[]}
        eventsTotal={0}
        onEventsPagerPrev={vi.fn()}
        onEventsPagerNext={vi.fn()}
      />,
    );

    expect(screen.getByRole("status", { name: /loading updates/i })).toBeInTheDocument();
  });

  it("shows updates error callout with retry wired to refetch", async () => {
    const user = userEvent.setup();
    const refetch = vi.fn().mockResolvedValue({ data: undefined });
    render(
      <TaskDetailUpdatesSection
        taskId="t1"
        eventsQuery={asEventsQuery({
          isPending: false,
          isError: true,
          error: new Error("events unavailable"),
          data: undefined,
          refetch,
        })}
        timelineEvents={[]}
        eventsTotal={0}
        onEventsPagerPrev={vi.fn()}
        onEventsPagerNext={vi.fn()}
      />,
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/events unavailable/i);
    await user.click(screen.getByRole("button", { name: /try again/i }));
    expect(refetch).toHaveBeenCalledTimes(1);
  });

  it("shows pager when the server reports more pages", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailUpdatesSection
          taskId="t1"
          eventsQuery={asEventsQuery({
            isPending: false,
            isError: false,
            error: null,
            data: {
              task_id: "t1",
              events: [sampleEvent],
              limit: 20,
              total: 40,
              range_start: 1,
              range_end: 20,
              has_more_newer: true,
              has_more_older: false,
              approval_pending: false,
            },
          })}
          timelineEvents={[sampleEvent]}
          eventsTotal={40}
          onEventsPagerPrev={vi.fn()}
          onEventsPagerNext={vi.fn()}
        />
      </MemoryRouter>,
    );

    expect(
      screen.getByRole("navigation", { name: /update history pages/i }),
    ).toBeInTheDocument();
    expect(screen.getByText("1–20 of 40")).toBeInTheDocument();
  });
});
