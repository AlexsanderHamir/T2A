import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { TaskListSection } from "./TaskListSection";

describe("TaskListSection", () => {
  it("shows loading status", () => {
    render(
      <TaskListSection
        tasks={[]}
        loading
        refreshing={false}
        saving={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });

  it("shows syncing status when refreshing", () => {
    render(
      <TaskListSection
        tasks={[]}
        loading={false}
        refreshing
        saving={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("Syncing with server…")).toBeInTheDocument();
  });

  it("shows empty copy when not loading and no tasks", () => {
    render(
      <TaskListSection
        tasks={[]}
        loading={false}
        refreshing={false}
        saving={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("No tasks yet.")).toBeInTheDocument();
  });

  it("renders rows and calls onEdit", async () => {
    const user = userEvent.setup();
    const onEdit = vi.fn();
    const onRequestDelete = vi.fn();
    const task = {
      id: "1",
      title: "Alpha",
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
    };
    render(
      <TaskListSection
        tasks={[task]}
        loading={false}
        refreshing={false}
        saving={false}
        onEdit={onEdit}
        onRequestDelete={onRequestDelete}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^edit$/i }));
    expect(onEdit).toHaveBeenCalledWith(task);
    await user.click(screen.getByRole("button", { name: /^delete$/i }));
    expect(onRequestDelete).toHaveBeenCalledWith(task);
  });
});
