import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import type { useTasksApp } from "../hooks/useTasksApp";
import { TasksAppProvider } from "../app/TasksAppProvider";
import { TaskDraftsPage } from "./TaskDraftsPage";

type App = ReturnType<typeof useTasksApp>;

function makeApp(overrides: Partial<App> = {}): App {
  return {
    taskDrafts: [],
    draftListLoading: false,
    draftListError: null,
    retryDraftList: vi.fn(),
    resumeDraftByID: vi.fn().mockResolvedValue(undefined),
    resumeDraftPending: false,
    resumeDraftError: null,
    deleteDraftByID: vi.fn().mockResolvedValue(undefined),
    deleteDraftPending: false,
    deleteDraftError: null,
    openCreateModal: vi.fn(),
    ...overrides,
  } as unknown as App;
}

function renderDraftsPage(app: App) {
  return render(
    <TasksAppProvider value={app}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={["/drafts"]}>
        <Routes>
          <Route path="/" element={<div>Home route</div>} />
          <Route path="/drafts" element={<TaskDraftsPage />} />
        </Routes>
      </MemoryRouter>
    </TasksAppProvider>,
  );
}

describe("TaskDraftsPage", () => {
  it("shows loading skeleton while the draft list is pending", async () => {
    vi.useFakeTimers();
    try {
      renderDraftsPage(makeApp({ draftListLoading: true }));

      expect(screen.getByRole("heading", { name: /^task drafts$/i })).toBeInTheDocument();
      expect(screen.queryByRole("status", { name: /loading drafts/i })).not.toBeInTheDocument();

      await vi.advanceTimersByTimeAsync(300);

      expect(screen.getByRole("status", { name: /loading drafts/i })).toBeInTheDocument();
    } finally {
      vi.useRealTimers();
    }
  });

  it("shows an error with try again when the draft list fails", async () => {
    const retryDraftList = vi.fn();
    renderDraftsPage(
      makeApp({
        draftListError: "drafts unavailable",
        retryDraftList,
      }),
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/drafts unavailable/i);
    fireEvent.click(screen.getByRole("button", { name: /^try again$/i }));
    expect(retryDraftList).toHaveBeenCalledTimes(1);
  });

  it("navigates home and opens create modal from the empty state", async () => {
    const openCreateModal = vi.fn();
    renderDraftsPage(makeApp({ openCreateModal }));

    fireEvent.click(screen.getByRole("button", { name: /^create a task$/i }));

    expect(screen.getByText("Home route")).toBeInTheDocument();
    expect(openCreateModal).toHaveBeenCalledTimes(1);
  });

  it("shows resume error from app state", () => {
    renderDraftsPage(
      makeApp({
        taskDrafts: [
          {
            id: "d1",
            name: "Broken draft",
            created_at: "2026-04-07T10:00:00Z",
            updated_at: "2026-04-07T10:05:00Z",
          },
        ],
        resumeDraftError: "resume failed",
      }),
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/resume failed/i);
  });

  it("shows delete error from app state", () => {
    renderDraftsPage(
      makeApp({
        taskDrafts: [
          {
            id: "d1",
            name: "Delete me",
            created_at: "2026-04-07T10:00:00Z",
            updated_at: "2026-04-07T10:05:00Z",
          },
        ],
        deleteDraftError: "delete failed",
      }),
    );

    expect(screen.getByRole("alert")).toHaveTextContent(/delete failed/i);
  });

  it("calls resumeDraftByID when a draft row is opened", async () => {
    const user = userEvent.setup();
    const resumeDraftByID = vi.fn().mockResolvedValue(undefined);
    renderDraftsPage(
      makeApp({
        resumeDraftByID,
        taskDrafts: [
          {
            id: "d1",
            name: "Draft from list",
            created_at: "2026-04-07T10:00:00Z",
            updated_at: "2026-04-07T10:05:00Z",
          },
        ],
      }),
    );

    await user.click(
      screen.getByRole("listitem", { name: /^resume draft: draft from list$/i }),
    );

    expect(resumeDraftByID).toHaveBeenCalledWith("d1");
  });
});
