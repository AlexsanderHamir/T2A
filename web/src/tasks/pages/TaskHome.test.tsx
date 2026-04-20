import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import type { useTasksApp } from "../hooks/useTasksApp";
import type { TaskStatsResponse } from "@/types/task";
import { TaskHome } from "./TaskHome";

vi.mock("../components/task-list", () => ({
  TaskListSection: () => <div data-testid="task-list-section" />,
}));

vi.mock("../components/task-create-modal", () => ({
  TaskCreateModal: () => null,
}));

vi.mock("../components/draft-resume", () => ({
  DraftResumeModal: () => null,
}));

type App = ReturnType<typeof useTasksApp>;

function makeApp(overrides: Partial<App> = {}): App {
  return {
    tasks: [],
    rootTasksOnPage: [],
    loading: false,
    listRefreshing: false,
    saving: false,
    sseLive: false,
    taskListPage: 0,
    taskListPageSize: 50,
    hasNextTaskPage: false,
    hasPrevTaskPage: false,
    setTaskListPage: () => {},
    resetTaskListPage: () => {},
    openEdit: () => {},
    requestDelete: () => {},
    openCreateModal: () => {},
    closeCreateModal: () => {},
    createModalOpen: false,
    createEntryDraftErrorHint: false,
    retryCreateEntryDraftLoad: () => {},
    draftPickerOpen: false,
    setDraftPickerOpen: () => {},
    taskDrafts: [],
    draftListLoading: false,
    draftListError: null,
    retryDraftList: () => {},
    startFreshDraft: async () => {},
    resumeDraftByID: async () => {},
    resumeDraftPending: false,
    resumeDraftError: null,
    taskStats: undefined,
    taskStatsLoading: true,
    ...overrides,
  } as unknown as App;
}

function statsFixture(overrides: Partial<TaskStatsResponse> = {}): TaskStatsResponse {
  return {
    total: 12,
    ready: 4,
    critical: 2,
    by_status: { ready: 4 },
    by_priority: { critical: 2 },
    by_scope: { parent: 7, subtask: 5 },
    ...overrides,
  } as TaskStatsResponse;
}

function renderHome(app: App) {
  return render(
    <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
      <TaskHome app={app} />
    </MemoryRouter>,
  );
}

describe("TaskHome KPI cards", () => {
  it("shows skeleton placeholders while taskStats is loading on first fetch", () => {
    renderHome(makeApp({ taskStats: undefined, taskStatsLoading: true }));

    const overview = screen.getByLabelText("Task overview");
    const cards = within(overview).getAllByRole("article");
    expect(cards).toHaveLength(3);
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "true");
    });

    expect(within(overview).getByText("Loading Total tasks")).toBeInTheDocument();
    expect(within(overview).getByText("Loading Ready tasks")).toBeInTheDocument();
    expect(within(overview).getByText("Loading Critical tasks")).toBeInTheDocument();
    expect(within(overview).getByText("Loading breakdown…")).toBeInTheDocument();

    expect(within(overview).queryByText("12")).not.toBeInTheDocument();
  });

  it("shows real stats values once the query resolves", () => {
    renderHome(
      makeApp({
        taskStats: statsFixture(),
        taskStatsLoading: false,
      }),
    );

    const overview = screen.getByLabelText("Task overview");
    const cards = within(overview).getAllByRole("article");
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "false");
    });

    expect(within(overview).getByText("12")).toBeInTheDocument();
    expect(within(overview).getByText("4")).toBeInTheDocument();
    expect(within(overview).getByText("2")).toBeInTheDocument();
    expect(within(overview).getByText("7 parent • 5 subtasks")).toBeInTheDocument();
  });

  it("uses singular subtask noun when by_scope.subtask is 1", () => {
    renderHome(
      makeApp({
        taskStats: statsFixture({
          by_scope: { parent: 3, subtask: 1 },
        }),
        taskStatsLoading: false,
      }),
    );

    const overview = screen.getByLabelText("Task overview");
    expect(within(overview).getByText("3 parent • 1 subtask")).toBeInTheDocument();
  });

  it("shows '—' with an unavailable hint when stats settled to null", () => {
    renderHome(
      makeApp({
        taskStats: null as unknown as TaskStatsResponse,
        taskStatsLoading: false,
      }),
    );

    const overview = screen.getByLabelText("Task overview");
    const cards = within(overview).getAllByRole("article");
    cards.forEach((card) => {
      expect(card).toHaveAttribute("aria-busy", "false");
    });

    const dashes = within(overview).getAllByText("—");
    expect(dashes).toHaveLength(3);
    expect(within(overview).getByLabelText("Total tasks unavailable")).toBeInTheDocument();
    expect(within(overview).getByLabelText("Ready tasks unavailable")).toBeInTheDocument();
    expect(within(overview).getByLabelText("Critical tasks unavailable")).toBeInTheDocument();
    expect(within(overview).getByText("Breakdown unavailable")).toBeInTheDocument();
  });

  it("does NOT fall back to client-paged tasks for KPI numerals", () => {
    renderHome(
      makeApp({
        taskStats: undefined,
        taskStatsLoading: true,
        tasks: [
          { id: "a", title: "a", status: "ready", priority: "critical" },
          { id: "b", title: "b", status: "ready", priority: "low" },
          { id: "c", title: "c", status: "blocked", priority: "low" },
        ] as unknown as App["tasks"],
      }),
    );

    const overview = screen.getByLabelText("Task overview");
    expect(within(overview).queryByText("3")).not.toBeInTheDocument();
    expect(within(overview).queryByText("2")).not.toBeInTheDocument();
    expect(within(overview).queryByText("1")).not.toBeInTheDocument();
  });
});
