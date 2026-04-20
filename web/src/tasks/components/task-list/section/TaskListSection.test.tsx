import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactElement } from "react";
import { MemoryRouter } from "react-router-dom";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ROUTER_FUTURE_FLAGS } from "../../../../lib/routerFutureFlags";
import { TASK_TEST_DEFAULTS } from "@/test/taskDefaults";
import { TaskListSection } from "./TaskListSection";

const { mockPatchTask, mockDeleteTask } = vi.hoisted(() => ({
  mockPatchTask: vi.fn(),
  mockDeleteTask: vi.fn(),
}));

vi.mock("@/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/api")>();
  return {
    ...actual,
    patchTask: mockPatchTask,
    deleteTask: mockDeleteTask,
  };
});

beforeEach(() => {
  mockPatchTask.mockReset();
  mockDeleteTask.mockReset();
  mockDeleteTask.mockResolvedValue(undefined);
});

afterEach(() => {
  vi.restoreAllMocks();
});

function renderWithRouter(ui: ReactElement) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  });
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>{ui}</MemoryRouter>
    </QueryClientProvider>,
  );
}

function makeRow(
  id: string,
  title: string,
  extras: Partial<{
    status: import("@/types").Status;
    priority: import("@/types").Priority;
    pickup_not_before?: string;
  }> = {},
) {
  return {
    id,
    title,
    initial_prompt: "",
    status: extras.status ?? ("ready" as const),
    priority: extras.priority ?? ("medium" as const),
    checklist_inherit: false as const,
    pickup_not_before: extras.pickup_not_before,
    ...TASK_TEST_DEFAULTS,
    depth: 0,
  };
}

const listPagerDefaults = {
  listPage: 0,
  listPageSize: 20,
  onListPageChange: vi.fn(),
  onListFiltersChange: vi.fn(),
  hasNextPage: false,
  hasPrevPage: false,
  rootTasksOnPage: 0,
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
    expect(
      screen.getByRole("status", { name: "Loading tasks" }),
    ).toBeInTheDocument();
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
    expect(screen.getByText(/no tasks yet/i)).toBeInTheDocument();
    expect(
      screen.getByRole("region", { name: /^all tasks$/i }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("table", {
        name: /all tasks: title, status, priority, prompt preview, and row actions/i,
      }),
    ).toBeInTheDocument();
  });

  it("calls emptyListAction when the empty-state CTA is used", async () => {
    const user = userEvent.setup();
    const onCreate = vi.fn();
    renderWithRouter(
      <TaskListSection
        tasks={[]}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
        emptyListAction={{
          label: "Create one",
          onClick: onCreate,
        }}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^create one$/i }));
    expect(onCreate).toHaveBeenCalledTimes(1);
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
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
      depth: 0,
    };
    renderWithRouter(
      <TaskListSection
        tasks={[task]}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
        rootTasksOnPage={1}
        onEdit={onEdit}
        onRequestDelete={onRequestDelete}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: /^edit task "alpha"$/i }),
    );
    expect(onEdit).toHaveBeenCalledWith(task);
    await user.click(
      screen.getByRole("button", { name: /^delete task "alpha"$/i }),
    );
    expect(onRequestDelete).toHaveBeenCalledWith({
      ...task,
      subtaskCount: 0,
    });
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
        checklist_inherit: false as const,
        ...TASK_TEST_DEFAULTS,
        depth: 0,
      },
      {
        id: "2",
        title: "High done",
        initial_prompt: "",
        status: "done" as const,
        priority: "high" as const,
        checklist_inherit: false as const,
        ...TASK_TEST_DEFAULTS,
        depth: 0,
      },
    ];
    renderWithRouter(
      <TaskListSection
        tasks={tasks}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
        rootTasksOnPage={2}
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
        checklist_inherit: false as const,
        ...TASK_TEST_DEFAULTS,
        depth: 0,
      },
      {
        id: "2",
        title: "Beta",
        initial_prompt: "",
        status: "ready" as const,
        priority: "medium" as const,
        checklist_inherit: false as const,
        ...TASK_TEST_DEFAULTS,
        depth: 0,
      },
    ];
    renderWithRouter(
      <TaskListSection
        tasks={tasks}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
        rootTasksOnPage={2}
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
            checklist_inherit: false as const,
            ...TASK_TEST_DEFAULTS,
            depth: 0,
          },
        ]}
        loading={false}
        refreshing={false}
        saving={false}
        {...listPagerDefaults}
        rootTasksOnPage={1}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("combobox", { name: /^status$/i }));
    await user.click(screen.getByRole("option", { name: /^failed$/i }));
    expect(screen.getByText(/no matching tasks/i)).toBeInTheDocument();
  });

  describe("bulk reschedule", () => {
    it("disables Reschedule when any selected task is done", async () => {
      const user = userEvent.setup();
      const tasks = [
        makeRow("a", "Alpha", { status: "done" }),
        makeRow("b", "Beta", { status: "ready" }),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={2}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-row-a"));
      const reschedule = screen.getByTestId("task-list-bulk-bar-reschedule");
      expect(reschedule).toBeDisabled();
      expect(reschedule).toHaveAttribute(
        "title",
        "Completed tasks cannot be rescheduled from the list.",
      );
    });

    it("shows the bulk action bar only after at least one row is selected", async () => {
      const user = userEvent.setup();
      const tasks = [
        makeRow("a", "Alpha"),
        makeRow("b", "Beta"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={2}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      expect(
        screen.queryByTestId("task-list-bulk-bar"),
      ).not.toBeInTheDocument();
      await user.click(screen.getByTestId("task-list-select-row-a"));
      const bar = screen.getByTestId("task-list-bulk-bar");
      expect(bar).toBeInTheDocument();
      expect(
        within(bar).getByTestId("task-list-bulk-bar-summary"),
      ).toHaveTextContent("1 task selected");
    });

    it("select-all-visible toggles every row in the filtered set", async () => {
      const user = userEvent.setup();
      const tasks = [
        makeRow("a", "Alpha"),
        makeRow("b", "Beta"),
        makeRow("c", "Gamma"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={3}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-all"));
      const bar = screen.getByTestId("task-list-bulk-bar");
      expect(
        within(bar).getByTestId("task-list-bulk-bar-summary"),
      ).toHaveTextContent("3 tasks selected");
      await user.click(screen.getByTestId("task-list-select-all"));
      expect(
        screen.queryByTestId("task-list-bulk-bar"),
      ).not.toBeInTheDocument();
    });

    it("Reschedule bulk action PATCHes every selected task with the same ISO", async () => {
      const user = userEvent.setup();
      mockPatchTask.mockImplementation(async (id: string, body: { pickup_not_before?: string | null }) => ({
        id,
        title: id,
        initial_prompt: "",
        status: "ready",
        priority: "medium",
        checklist_inherit: false,
        pickup_not_before: body.pickup_not_before ?? undefined,
        ...TASK_TEST_DEFAULTS,
      }));
      const tasks = [
        makeRow("a", "Alpha"),
        makeRow("b", "Beta"),
        makeRow("c", "Gamma"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={3}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-row-a"));
      await user.click(screen.getByTestId("task-list-select-row-b"));
      await user.click(screen.getByTestId("task-list-select-row-c"));
      await user.click(screen.getByTestId("task-list-bulk-bar-reschedule"));
      const fixedNow = Date.UTC(2026, 3, 19, 12, 0, 0);
      const dateNow = vi.spyOn(Date, "now").mockReturnValue(fixedNow);
      await user.click(screen.getByTestId("schedule-picker-in-1h"));
      dateNow.mockRestore();
      await user.click(screen.getByTestId("task-bulk-reschedule-submit"));
      await waitFor(() => {
        expect(mockPatchTask).toHaveBeenCalledTimes(3);
      });
      const calls = mockPatchTask.mock.calls;
      const ids = calls.map((c) => c[0]).sort();
      expect(ids).toEqual(["a", "b", "c"]);
      const bodies = calls.map((c) => c[1]);
      const isoSet = new Set(
        bodies.map((b) => (b as { pickup_not_before: string }).pickup_not_before),
      );
      expect(isoSet.size).toBe(1);
      const iso = [...isoSet][0];
      expect(typeof iso).toBe("string");
      expect(new Date(iso).toISOString()).toBe(iso);
      await waitFor(() => {
        expect(
          screen.queryByTestId("task-list-bulk-bar"),
        ).not.toBeInTheDocument();
      });
    });

    it("Clear schedule bulk action sends pickup_not_before=null to scheduled rows only", async () => {
      const user = userEvent.setup();
      mockPatchTask.mockResolvedValue({
        id: "a",
        title: "Alpha",
        initial_prompt: "",
        status: "ready",
        priority: "medium",
        checklist_inherit: false,
        ...TASK_TEST_DEFAULTS,
      });
      const futureIso = new Date(Date.now() + 60 * 60 * 1000).toISOString();
      const tasks = [
        makeRow("a", "Alpha", { pickup_not_before: futureIso }),
        makeRow("b", "Beta", { pickup_not_before: futureIso }),
        makeRow("c", "Gamma"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={3}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-all"));
      await user.click(screen.getByTestId("task-list-bulk-bar-clear"));
      await waitFor(() => {
        expect(mockPatchTask).toHaveBeenCalledTimes(2);
      });
      const ids = mockPatchTask.mock.calls.map((c) => c[0]).sort();
      expect(ids).toEqual(["a", "b"]);
      for (const c of mockPatchTask.mock.calls) {
        expect((c[1] as { pickup_not_before: unknown }).pickup_not_before).toBe(null);
      }
    });

    it("Clear schedule button is disabled when no selected row has a schedule", async () => {
      const user = userEvent.setup();
      const tasks = [makeRow("a", "Alpha"), makeRow("b", "Beta")];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={2}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-all"));
      const clearBtn = screen.getByTestId("task-list-bulk-bar-clear");
      expect(clearBtn).toBeDisabled();
    });

    it("partial failure surfaces an aggregate error and keeps the bar visible", async () => {
      const user = userEvent.setup();
      mockPatchTask
        .mockResolvedValueOnce({
          id: "a",
          title: "Alpha",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
          ...TASK_TEST_DEFAULTS,
        })
        .mockRejectedValueOnce(new Error("boom"))
        .mockResolvedValueOnce({
          id: "c",
          title: "Gamma",
          initial_prompt: "",
          status: "ready",
          priority: "medium",
          checklist_inherit: false,
          ...TASK_TEST_DEFAULTS,
        });
      const tasks = [
        makeRow("a", "Alpha"),
        makeRow("b", "Beta"),
        makeRow("c", "Gamma"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={3}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-all"));
      await user.click(screen.getByTestId("task-list-bulk-bar-reschedule"));
      const fixedNow = Date.UTC(2026, 3, 19, 12, 0, 0);
      const dateNow = vi.spyOn(Date, "now").mockReturnValue(fixedNow);
      await user.click(screen.getByTestId("schedule-picker-in-1h"));
      dateNow.mockRestore();
      await user.click(screen.getByTestId("task-bulk-reschedule-submit"));
      await waitFor(() => {
        expect(mockPatchTask).toHaveBeenCalledTimes(3);
      });
      // Modal closes only on full success — partial failure leaves
      // it open so the operator can retry.
      await waitFor(() => {
        const banner = screen.getByTestId("task-list-bulk-error");
        expect(banner.textContent).toMatch(/1 of 3/);
      });
    });

    it("Cancel clears the running selection without firing PATCHes", async () => {
      const user = userEvent.setup();
      const tasks = [makeRow("a", "Alpha"), makeRow("b", "Beta")];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={2}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-all"));
      await user.click(screen.getByTestId("task-list-bulk-bar-cancel"));
      expect(mockPatchTask).not.toHaveBeenCalled();
      expect(mockDeleteTask).not.toHaveBeenCalled();
      expect(
        screen.queryByTestId("task-list-bulk-bar"),
      ).not.toBeInTheDocument();
    });

    it("changing a filter clears the running selection", async () => {
      const user = userEvent.setup();
      const tasks = [
        makeRow("a", "Alpha", { status: "ready" }),
        makeRow("b", "Beta", { status: "done" }),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={2}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-row-a"));
      expect(screen.getByTestId("task-list-bulk-bar")).toBeInTheDocument();
      await user.click(screen.getByRole("combobox", { name: /^status$/i }));
      await user.click(screen.getByRole("option", { name: /^done$/i }));
      expect(
        screen.queryByTestId("task-list-bulk-bar"),
      ).not.toBeInTheDocument();
    });

    it("Scheduled (deferred) filter limits rows to ready+future", async () => {
      const user = userEvent.setup();
      const future = new Date(Date.now() + 60 * 60 * 1000).toISOString();
      const past = new Date(Date.now() - 60 * 60 * 1000).toISOString();
      const tasks = [
        makeRow("a", "Future ready", { pickup_not_before: future }),
        makeRow("b", "Past ready", { pickup_not_before: past }),
        makeRow("c", "Plain ready"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={3}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByRole("combobox", { name: /^status$/i }));
      await user.click(
        screen.getByRole("option", { name: /scheduled \(deferred\)/i }),
      );
      expect(screen.getByText("Future ready")).toBeInTheDocument();
      expect(screen.queryByText("Past ready")).not.toBeInTheDocument();
      expect(screen.queryByText("Plain ready")).not.toBeInTheDocument();
    });
  });

  describe("bulk delete", () => {
    it("opens confirm and DELETEs each selected task", async () => {
      const user = userEvent.setup();
      const tasks = [
        makeRow("a", "Alpha"),
        makeRow("b", "Beta"),
        makeRow("c", "Gamma"),
      ];
      renderWithRouter(
        <TaskListSection
          tasks={tasks}
          loading={false}
          refreshing={false}
          saving={false}
          {...listPagerDefaults}
          rootTasksOnPage={3}
          onEdit={vi.fn()}
          onRequestDelete={vi.fn()}
        />,
      );
      await user.click(screen.getByTestId("task-list-select-row-a"));
      await user.click(screen.getByTestId("task-list-select-row-b"));
      await user.click(screen.getByTestId("task-list-bulk-bar-delete"));
      expect(
        screen.getByRole("heading", { name: /delete 2 tasks/i }),
      ).toBeInTheDocument();
      await user.click(screen.getByTestId("task-bulk-delete-confirm"));
      await waitFor(() => {
        expect(mockDeleteTask).toHaveBeenCalledTimes(2);
      });
      const ids = mockDeleteTask.mock.calls.map((c) => c[0]).sort();
      expect(ids).toEqual(["a", "b"]);
      await waitFor(() => {
        expect(
          screen.queryByTestId("task-list-bulk-bar"),
        ).not.toBeInTheDocument();
      });
    });
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
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
      depth: 0,
    };
    const filler = Array.from({ length: 19 }, (_, i) => ({
      id: `x${i}`,
      title: `T${i}`,
      initial_prompt: "",
      status: "ready" as const,
      priority: "medium" as const,
      checklist_inherit: false as const,
      ...TASK_TEST_DEFAULTS,
      depth: 0,
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
        rootTasksOnPage={20}
        onEdit={vi.fn()}
        onRequestDelete={vi.fn()}
      />,
    );
    await user.click(screen.getByRole("button", { name: /^next$/i }));
    expect(onListPageChange).toHaveBeenCalledWith(1);
  });
});
