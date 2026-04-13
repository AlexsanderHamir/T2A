import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "../../lib/routerFutureFlags";
import { TaskDetailHeader } from "./TaskDetailHeader";

describe("TaskDetailHeader", () => {
  it("renders title, stance, status and priority pills, and back link", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailHeader
          task={{ title: "My task", status: "ready", priority: "high" }}
        />
      </MemoryRouter>,
    );

    expect(screen.getByRole("heading", { name: /^my task$/i })).toBeInTheDocument();
    expect(screen.getByText("Informational")).toHaveAttribute(
      "data-stance",
      "informational",
    );
    expect(screen.getByText("ready")).toBeInTheDocument();
    expect(screen.getByText("high")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /← all tasks/i })).toHaveAttribute(
      "href",
      "/",
    );
  });

  it("marks stance when status needs user input", () => {
    render(
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskDetailHeader
          task={{ title: "Blocked", status: "blocked", priority: "medium" }}
        />
      </MemoryRouter>,
    );

    expect(screen.getByText("Agent needs input")).toHaveAttribute(
      "data-stance",
      "needs-user",
    );
    expect(screen.getByText("blocked")).toHaveAttribute("data-needs-user", "true");
  });
});
