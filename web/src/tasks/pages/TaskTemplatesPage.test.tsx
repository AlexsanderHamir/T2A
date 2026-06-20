import { useQuery } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import type { useTasksApp } from "../hooks/useTasksApp";
import { TasksAppProvider } from "../app/TasksAppProvider";
import { TaskTemplatesPage } from "./TaskTemplatesPage";

vi.mock("@tanstack/react-query", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@tanstack/react-query")>();
  return {
    ...actual,
    useQuery: vi.fn(),
  };
});

const mockedUseQuery = vi.mocked(useQuery);

type App = ReturnType<typeof useTasksApp>;

function makeApp(overrides: Partial<App> = {}): App {
  return {
    openTemplateCreateModal: vi.fn(),
    editTemplateByID: vi.fn(),
    deleteTemplateByID: vi.fn().mockResolvedValue(undefined),
    instantiateTemplatesByIDs: vi.fn(),
    instantiateTemplatesPending: false,
    loadTemplatePending: false,
    deleteTemplatePending: false,
    ...overrides,
  } as unknown as App;
}

function renderPage(app: App) {
  return render(
    <TasksAppProvider value={app}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <TaskTemplatesPage />
      </MemoryRouter>
    </TasksAppProvider>,
  );
}

const templates = [
  {
    id: "tmpl-1",
    name: "Alpha template",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
  },
  {
    id: "tmpl-2",
    name: "Beta template",
    created_at: "2026-01-01T00:00:00Z",
    updated_at: "2026-01-02T00:00:00Z",
  },
];

describe("TaskTemplatesPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    mockedUseQuery.mockReturnValue({
      data: templates,
      isPending: false,
      isError: false,
      error: null,
      refetch: vi.fn(),
    } as ReturnType<typeof useQuery>);
  });

  it("lists templates from the query result", () => {
    renderPage(makeApp());
    expect(screen.getByText("Alpha template")).toBeInTheDocument();
    expect(screen.getByText("Beta template")).toBeInTheDocument();
  });

  it("shows batch action when rows are selected", async () => {
    const user = userEvent.setup();
    renderPage(makeApp());

    await user.click(screen.getByLabelText(/select alpha template/i));
    expect(screen.getByRole("button", { name: /create tasks \(1\)/i })).toBeInTheDocument();
  });

  it("calls instantiate with selected template ids in order", async () => {
    const user = userEvent.setup();
    const instantiateTemplatesByIDs = vi.fn().mockResolvedValue({ tasks: [{}], errors: [] });
    renderPage(makeApp({ instantiateTemplatesByIDs }));

    await user.click(screen.getByLabelText(/select alpha template/i));
    await user.click(screen.getByLabelText(/select beta template/i));
    await user.click(screen.getByRole("button", { name: /create tasks \(2\)/i }));

    await waitFor(() => {
      expect(instantiateTemplatesByIDs).toHaveBeenCalledWith(["tmpl-1", "tmpl-2"]);
    });
  });
});
