import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { TaskListSection } from "./TaskListSection";

function renderWithRouter(ui: ReactElement) {
  return render(<MemoryRouter>{ui}</MemoryRouter>);
}

const listPagerDefaults = {
  listPage: 0,
  listPageSize: 20,
  onListPageChange: vi.fn(),
  onListFiltersChange: vi.fn(),
  hasNextPage: false,
  hasPrevPage: false,
};

describe("TaskListSection", () => {
  it("shows loading status", () => {
    renderWithRouter(
      <TaskListSection
        tasks={[]}
        loading
        refreshing={false}
        saving={false}
        smoothTransitions={false}
        {...listPagerDefaults}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByRole("status")).toHaveTextContent("Loading…");
  });

  it("shows syncing status when refreshing", () => {
    renderWithRouter(
      <TaskListSection
        tasks={[]}
        loading={false}
        refreshing
        saving={false}
        smoothTransitions={false}
        {...listPagerDefaults}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("Syncing with server…")).toBeInTheDocument();
  });

  it("shows empty copy when not loading and no tasks", () => {
    renderWithRouter(
      <TaskListSection
        tasks={[]}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
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
    renderWithRouter(
      <TaskListSection
        tasks={[task]}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
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
    renderWithRouter(
      <TaskListSection
        tasks={tasks}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    expect(screen.getByText("Low ready")).toBeInTheDocument();
    expect(screen.getByText("High done")).toBeInTheDocument();

    await user.click(screen.getByRole("combobox", { name: /^status$/i }));
    await user.click(screen.getByRole("option", { name: /^ready$/i }));
    expect(screen.getByText("Low ready")).toBeInTheDocument();
    expect(screen.queryByText("High done")).not.toBeInTheDocument();

    await user.click(screen.getByRole("combobox", { name: /^status$/i }));
    await user.click(screen.getByRole("option", { name: /^all$/i }));
    await user.click(screen.getByRole("combobox", { name: /^priority$/i }));
    await user.click(screen.getByRole("option", { name: /^high$/i }));
    expect(screen.queryByText("Low ready")).not.toBeInTheDocument();
    expect(screen.getByText("High done")).toBeInTheDocument();
  });

  it("shows status filter section labels for needs-user vs other", async () => {
    const user = userEvent.setup();
    renderWithRouter(
      <TaskListSection
        tasks={[]}
        loading={false}
        refreshing={false}
        saving={false}
        smoothTransitions={false}
        {...listPagerDefaults}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("combobox", { name: /^status$/i }));
    expect(screen.getByText("Agent needs input")).toBeInTheDocument();
    expect(screen.getByText("Other activity")).toBeInTheDocument();
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
    renderWithRouter(
      <TaskListSection
        tasks={tasks}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
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
    renderWithRouter(
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
        {...listPagerDefaults}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("combobox", { name: /^status$/i }));
    await user.click(screen.getByRole("option", { name: /^failed$/i }));
    expect(
      screen.getByText("No tasks match these filters."),
    ).toBeInTheDocument();
  });

  it("shows list pager when another server page may exist", async () => {
    const user = userEvent.setup();
    const onListPageChange = vi.fn();
    const task = {
      id: "1",
      title: "One",
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
    };
    const filler = Array.from({ length: 19 }, (_, i) => ({
      id: `x${i}`,
      title: `T${i}`,
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
    }));
    renderWithRouter(
      <TaskListSection
        tasks={[task, ...filler]}
        loading={false}
        refreshing={false}
        saving={false}
        listPage={0}
        listPageSize={20}
        onListPageChange={onListPageChange}
        onListFiltersChange={vi.fn()}
        hasNextPage
        hasPrevPage={false}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^next$/i }));
    expect(onListPageChange).toHaveBeenCalledWith(1);
  });
});
