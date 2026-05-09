import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { afterEach, describe, expect, it, vi } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { requestUrl } from "@/test/requestUrl";
import type { Project } from "@/types";
import { projectQueryKeys } from "./queryKeys";
import { ProjectStepsPage } from "./ProjectStepsPage";

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
    vi.spyOn(globalThis, "fetch").mockImplementation(async (input: FetchInput) => {
      const u = requestUrl(input);
      if (u.startsWith("/tasks?")) {
        return jsonResponse({ tasks: [], limit: 200, offset: 0, has_more: false });
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
      <QueryClientProvider client={queryClient}>
        <MemoryRouter
          future={ROUTER_FUTURE_FLAGS}
          initialEntries={[`/projects/${testProject.id}/steps?goal_id=goal-1`]}
        >
          <Routes>
            <Route path="/projects/:projectId/steps" element={<ProjectStepsPage />} />
          </Routes>
        </MemoryRouter>
      </QueryClientProvider>,
    );

    await waitFor(() => {
      expect(screen.getByRole("heading", { name: /Stages and completion/i })).toBeInTheDocument();
    });
    expect(screen.getByText("JWT implementation")).toBeInTheDocument();
    expect(screen.getByText("Add rotation job")).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Back to project/i })).toHaveAttribute(
      "href",
      `/projects/${testProject.id}`,
    );
  });
});
