import { render, screen } from "@testing-library/react";
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
    expect(screen.getByText(/loading history/i)).toBeInTheDocument();

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
});
