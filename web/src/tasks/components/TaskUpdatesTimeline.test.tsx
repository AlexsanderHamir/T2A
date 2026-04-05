import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { TaskUpdatesTimeline } from "./TaskUpdatesTimeline";

describe("TaskUpdatesTimeline", () => {
  it("renders newest-first rows and links list to heading", () => {
    render(
      <TaskUpdatesTimeline
        isPending={false}
        isError={false}
        error={null}
        isEmpty={false}
        timelineEvents={[
          {
            seq: 2,
            at: "2026-01-02T12:00:00.000Z",
            type: "sync_ping",
            by: "user",
            data: {},
          },
          {
            seq: 1,
            at: "2026-01-01T12:00:00.000Z",
            type: "task_created",
            by: "user",
            data: {},
          },
        ]}
      />,
    );

    const list = screen.getByRole("list", { name: /updates/i });
    expect(list).toHaveAttribute(
      "aria-labelledby",
      "task-detail-updates-heading",
    );
    const items = screen.getAllByRole("listitem");
    const pill0 = items[0].querySelector("code.task-timeline-type-pill");
    const pill1 = items[1].querySelector("code.task-timeline-type-pill");
    expect(pill0).toHaveTextContent("sync_ping");
    expect(pill0).toHaveAttribute("data-event-type", "sync_ping");
    expect(pill1).toHaveTextContent("task_created");
    expect(pill1).toHaveAttribute("data-event-type", "task_created");
  });

  it("shows type pill and payload for status_changed", () => {
    render(
      <TaskUpdatesTimeline
        isPending={false}
        isError={false}
        error={null}
        isEmpty={false}
        timelineEvents={[
          {
            seq: 1,
            at: "2026-01-01T12:00:00.000Z",
            type: "status_changed",
            by: "agent",
            data: { from: "ready", to: "running" },
          },
        ]}
      />,
    );
    const item = screen.getByRole("listitem");
    const pill = item.querySelector("code.task-timeline-type-pill");
    expect(pill).toHaveTextContent("status_changed");
    expect(pill).toHaveAttribute("aria-label", "Status changed, status_changed");
    expect(item).toHaveTextContent(/ready/);
    expect(item).toHaveTextContent(/running/);
  });

  it("shows loading and empty states", () => {
    const { rerender } = render(
      <TaskUpdatesTimeline
        isPending
        isError={false}
        error={null}
        isEmpty={false}
        timelineEvents={[]}
      />,
    );
    expect(
      screen.getByRole("status", { name: /loading updates/i }),
    ).toBeInTheDocument();

    rerender(
      <TaskUpdatesTimeline
        isPending={false}
        isError={false}
        error={null}
        isEmpty
        timelineEvents={[]}
      />,
    );
    expect(screen.getByText(/no audit events yet/i)).toBeInTheDocument();
  });

  it("uses one list for mixed events and marks needs-user rows", () => {
    render(
      <MemoryRouter>
        <TaskUpdatesTimeline
          isPending={false}
          isError={false}
          error={null}
          isEmpty={false}
          taskIdForLinks="t1"
          timelineEvents={[
            {
              seq: 3,
              at: "2026-01-03T12:00:00.000Z",
              type: "sync_ping",
              by: "user",
              data: {},
            },
            {
              seq: 2,
              at: "2026-01-02T12:00:00.000Z",
              type: "approval_requested",
              by: "agent",
              data: {},
            },
            {
              seq: 1,
              at: "2026-01-01T12:00:00.000Z",
              type: "task_failed",
              by: "agent",
              data: {},
            },
          ]}
        />
      </MemoryRouter>,
    );

    expect(screen.getAllByRole("list")).toHaveLength(1);
    const items = screen.getAllByRole("listitem");
    expect(items).toHaveLength(3);
    expect(items[0]).not.toHaveAttribute("data-needs-user");
    expect(items[1]).toHaveAttribute("data-needs-user", "true");
    expect(items[1]).toHaveClass("task-timeline-item--needs-user");
    expect(items[2]).toHaveAttribute("data-needs-user", "true");
    const approvalLink = screen.getByRole("link", {
      name: /approval requested — needs your input/i,
    });
    expect(approvalLink).toHaveAttribute("href", "/tasks/t1/events/2");
    expect(screen.getAllByText(/^Needs your input$/)).toHaveLength(2);
  });

  it("shows thread and hides needs-input hint when user replied last", () => {
    render(
      <MemoryRouter>
        <TaskUpdatesTimeline
          isPending={false}
          isError={false}
          error={null}
          isEmpty={false}
          taskIdForLinks="t1"
          timelineEvents={[
            {
              seq: 2,
              at: "2026-01-02T12:00:00.000Z",
              type: "approval_requested",
              by: "agent",
              data: {},
              response_thread: [
                {
                  at: "2026-01-02T12:00:00.000Z",
                  by: "agent",
                  body: "Please confirm",
                },
                {
                  at: "2026-01-02T12:05:00.000Z",
                  by: "user",
                  body: "Confirmed",
                },
              ],
            },
          ]}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText(/^Conversation$/)).toBeInTheDocument();
    expect(screen.getByText("Confirmed")).toBeInTheDocument();
    expect(screen.queryByText(/^Needs your input$/)).toBeNull();
    expect(
      screen.getByRole("link", {
        name: /approval requested — waiting on agent/i,
      }),
    ).toBeInTheDocument();
  });

  it("links each row when taskIdForLinks is set", () => {
    render(
      <MemoryRouter>
        <TaskUpdatesTimeline
          isPending={false}
          isError={false}
          error={null}
          isEmpty={false}
          taskIdForLinks="abc-task"
          timelineEvents={[
            {
              seq: 5,
              at: "2026-01-02T12:00:00.000Z",
              type: "sync_ping",
              by: "user",
              data: {},
            },
          ]}
        />
      </MemoryRouter>,
    );

    const link = screen.getByRole("link");
    expect(link).toHaveAttribute("href", "/tasks/abc-task/events/5");
  });
});
