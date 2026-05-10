import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { requestUrl } from "@/test/requestUrl";
import type { Project } from "@/types";
import type { useTasksApp } from "@/tasks/hooks/useTasksApp";
import { projectQueryKeys } from "./queryKeys";
import { ProjectStepsPage } from "./ProjectStepsPage";

type TasksApp = ReturnType<typeof useTasksApp>;

function makeTasksApp(overrides: Partial<TasksApp> = {}): TasksApp {
  return {
    openCreateModal: vi.fn(),
    createModalOpen: false,
    draftPickerOpen: false,
    ...overrides,
  } as unknown as TasksApp;
}

type FetchInput = RequestInfo | URL;

function jsonResponse(body: unknown, init: ResponseInit = { status: 200 }): Response {
  return new Response(JSON.stringify(body), {
    ...init,
    headers: { "content-type": "application/json", ...(init.headers ?? {}) },
  });
}

const testProject: Project = {
  id: "project-1",
  name: "AuthV2",
  description: "Auth refactor",
  status: "active",
  context_summary: "",
  created_at: "2026-04-27T00:00:00Z",
  updated_at: "2026-04-27T00:00:00Z",
};

describe("ProjectStepsPage", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("renders the steps workspace for a project", async () => {
    const openCreateModal = vi.fn();
    const tasksApp = makeTasksApp({ openCreateModal });

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput) => {
      const u = requestUrl(input);
      if (u.startsWith("/tasks?")) {
        return jsonResponse({ tasks: [], limit: 200, offset: 0, has_more: false });
      }
      if (
        u.includes(`/projects/${testProject.id}`) &&
        !u.includes("/steps") &&
        !u.includes("/context") &&
        !u.includes("/goals")
      ) {
        return jsonResponse(testProject);
      }
      if (u.includes(`/projects/${testProject.id}/goals`) && !u.match(/\/goals\/[^/?]+/)) {
        return jsonResponse({
          goals: [
            {
              id: "goal-1",
              project_id: testProject.id,
              title: "Security milestone",
              description: "",
              depends_on_goal_ids: [],
              gate_status: "active",
              gate_hold: false,
              criteria: [],
              created_at: "2026-01-01T00:00:00Z",
              updated_at: "2026-01-01T00:00:00Z",
            },
          ],
        });
      }
      return new Response(`unexpected ${u}`, { status: 500 });
    });

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: 0, staleTime: Infinity },
        mutations: { retry: false },
      },
    });
    queryClient.setQueryData(projectQueryKeys.detail(testProject.id), testProject);
    queryClient.setQueryData(projectQueryKeys.goals(testProject.id), {
      goals: [
        {
          id: "goal-1",
          project_id: testProject.id,
          title: "Security milestone",
          description: "",
          depends_on_goal_ids: [],
          gate_status: "active",
          gate_hold: false,
          criteria: [],
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      ],
    });
    queryClient.setQueryData(projectQueryKeys.steps(testProject.id, "goal-1"), {
      steps: [
        {
          id: "step-1",
          project_id: testProject.id,
          goal_id: "goal-1",
          title: "JWT implementation",
          description: "Refresh + rotation",
          sort_order: 1,
          gate_status: "active",
          gate_hold: false,
          criteria: [
            {
              id: "c1",
              text: "Add rotation job",
              done: false,
              sort_order: 1,
            },
          ],
          created_at: "2026-01-01T00:00:00Z",
          updated_at: "2026-01-01T00:00:00Z",
        },
      ],
    });

    render(
      <ModalStackProvider>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter
            future={ROUTER_FUTURE_FLAGS}
            initialEntries={[`/projects/${testProject.id}/steps?goal_id=goal-1`]}
          >
            <Routes>
              <Route
                path="/projects/:projectId/steps"
                element={<ProjectStepsPage app={tasksApp} />}
              />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>
      </ModalStackProvider>,
    );

    await waitFor(() => {
      expect(
        screen.getByRole("heading", { name: "Security milestone · Steps", hidden: true }),
      ).toBeInTheDocument();
    });
    expect(screen.getAllByText("Security milestone").length).toBeGreaterThanOrEqual(1);
    expect(screen.getByText("JWT implementation")).toBeInTheDocument();
    expect(screen.getByText("Add rotation job")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Back to goals/i })).toHaveAttribute(
      "href",
      `/projects/${testProject.id}/goals`,
    );

    await userEvent.click(screen.getByRole("button", { name: /^New task$/ }));
    expect(openCreateModal).toHaveBeenCalledWith({
      projectID: testProject.id,
      projectStepID: "step-1",
      lockProjectAssignment: true,
    });

    await userEvent.click(screen.getByRole("button", { name: /^Add step$/ }));
    expect(screen.getByRole("dialog")).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: /Add a step/i })).toBeInTheDocument();
  });

  it("shows Back to project when steps route has no goal_id", async () => {
    const tasksApp = makeTasksApp();

    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput) => {
      const u = requestUrl(input);
      if (u.startsWith("/tasks?")) {
        return jsonResponse({ tasks: [], limit: 200, offset: 0, has_more: false });
      }
      if (u.includes(`/projects/${testProject.id}/goals`)) {
        return jsonResponse({ goals: [] });
      }
      if (
        u.includes(`/projects/${testProject.id}`) &&
        !u.includes("/steps") &&
        !u.includes("/context")
      ) {
        return jsonResponse(testProject);
      }
      return new Response(`unexpected ${u}`, { status: 500 });
    });

    const queryClient = new QueryClient({
      defaultOptions: {
        queries: { retry: false, gcTime: 0, staleTime: Infinity },
        mutations: { retry: false },
      },
    });
    queryClient.setQueryData(projectQueryKeys.detail(testProject.id), testProject);

    render(
      <ModalStackProvider>
        <QueryClientProvider client={queryClient}>
          <MemoryRouter
            future={ROUTER_FUTURE_FLAGS}
            initialEntries={[`/projects/${testProject.id}/steps`]}
          >
            <Routes>
              <Route
                path="/projects/:projectId/steps"
                element={<ProjectStepsPage app={tasksApp} />}
              />
            </Routes>
          </MemoryRouter>
        </QueryClientProvider>
      </ModalStackProvider>,
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: "Choose a goal" })).toBeInTheDocument();
    });
    expect(screen.getByRole("link", { name: /Back to project/i })).toHaveAttribute(
      "href",
      `/projects/${testProject.id}`,
    );
  });
});
