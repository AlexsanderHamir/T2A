import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import type { useTasksApp } from "../hooks/useTasksApp";
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

function renderHome(app: App) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false, staleTime: Infinity } },
  });
  return render(
    <QueryClientProvider client={client}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskHome app={app} />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("TaskHome", () => {
  it("renders the task list without KPI stats cards", () => {
    renderHome(makeApp());

    expect(screen.getByTestId("task-list-section")).toBeInTheDocument();
    expect(screen.queryByLabelText("Task overview")).not.toBeInTheDocument();
    expect(screen.queryByText(/total tasks/i)).not.toBeInTheDocument();
  });
});
