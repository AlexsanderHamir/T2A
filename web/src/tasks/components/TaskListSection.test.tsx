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

  it("filters rows by status and priority", async () => {
    const user = userEvent.setup();
    const tasks = [
      {
        id: "1",
        title: "Low ready",
        initial_prompt: "",
        status: "ready" as const,
        priority: "low" as const,
      },
      {
        id: "2",
        title: "High done",
        initial_prompt: "",
        status: "done" as const,
        priority: "high" as const,
      },
    ];
    render(
      <TaskListSection
        tasks={tasks}
        loading={false}
        refreshing={false}
        saving={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("Low ready")).toBeInTheDocument();
    expect(screen.getByText("High done")).toBeInTheDocument();

    await user.selectOptions(
      screen.getByLabelText(/^status$/i),
      "ready",
    );
    expect(screen.getByText("Low ready")).toBeInTheDocument();
    expect(screen.queryByText("High done")).not.toBeInTheDocument();

    await user.selectOptions(screen.getByLabelText(/^status$/i), "all");
    await user.selectOptions(
      screen.getByLabelText(/^priority$/i),
      "high",
    );
    expect(screen.queryByText("Low ready")).not.toBeInTheDocument();
    expect(screen.getByText("High done")).toBeInTheDocument();
  });

  it("filters rows by title search", async () => {
    const user = userEvent.setup();
    const tasks = [
      {
        id: "1",
        title: "Alpha task",
        initial_prompt: "",
        status: "ready" as const,
        priority: "medium" as const,
      },
      {
        id: "2",
        title: "Beta",
        initial_prompt: "",
        status: "ready" as const,
        priority: "medium" as const,
      },
    ];
    render(
      <TaskListSection
        tasks={tasks}
        loading={false}
        refreshing={false}
        saving={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("Alpha task")).toBeInTheDocument();
    expect(screen.getByText("Beta")).toBeInTheDocument();

    const search = screen.getByLabelText(/^search titles$/i);
    await user.type(search, "alp");
    expect(screen.getByText("Alpha task")).toBeInTheDocument();
    expect(screen.queryByText("Beta")).not.toBeInTheDocument();

    await user.clear(search);
    expect(screen.getByText("Beta")).toBeInTheDocument();
  });

  it("shows copy when no tasks match filters", async () => {
    const user = userEvent.setup();
    render(
      <TaskListSection
        tasks={[
          {
            id: "1",
            title: "Only ready",
            initial_prompt: "",
            status: "ready" as const,
            priority: "medium" as const,
          },
        ]}
        loading={false}
        refreshing={false}
        saving={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    await user.selectOptions(screen.getByLabelText(/^status$/i), "failed");
    expect(
      screen.getByText("No tasks match these filters."),
    ).toBeInTheDocument();
  });
});
