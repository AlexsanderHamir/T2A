import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { TaskDependenciesPanel } from "./TaskDependenciesPanel";

describe("TaskDependenciesPanel", () => {
  it("shows empty state when there are no dependencies", () => {
    render(
      <MemoryRouter>
        <TaskDependenciesPanel dependencies={[]} />
      </MemoryRouter>,
    );
    expect(screen.getByTestId("task-deps-empty")).toBeInTheDocument();
  });

  it("lists dependencies with status pills", () => {
    render(
      <MemoryRouter>
        <TaskDependenciesPanel
          dependencies={[
            { id: "d1", title: "Upstream", status: "done" },
            { id: "d2", title: "Blocker", status: "running" },
          ]}
        />
      </MemoryRouter>,
    );
    expect(screen.getByTestId("task-deps-list")).toBeInTheDocument();
    expect(screen.getByText("Upstream")).toBeInTheDocument();
    expect(screen.getByText("done")).toBeInTheDocument();
    expect(screen.getByText("running")).toBeInTheDocument();
  });

  // Dependencies are fixed at creation time — the detail view is read-only.
  // Guard against the add/remove affordances creeping back in. (2026-06-04)
  it("offers no add or remove affordances (read-only)", () => {
    render(
      <MemoryRouter>
        <TaskDependenciesPanel
          dependencies={[{ id: "d1", title: "Upstream", status: "done" }]}
        />
      </MemoryRouter>,
    );
    expect(
      screen.queryByRole("button", { name: /^add$/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /^remove$/i }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByLabelText(/add dependency/i),
    ).not.toBeInTheDocument();
  });
});
