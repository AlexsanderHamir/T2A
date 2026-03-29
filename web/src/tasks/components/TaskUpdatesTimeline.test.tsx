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
    expect(items[0]).toHaveTextContent(/live sync check/i);
    expect(items[1]).toHaveTextContent(/task created/i);
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
