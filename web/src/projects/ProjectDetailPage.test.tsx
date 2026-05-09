import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen } from "@testing-library/react";
import { MemoryRouter, Route, Routes } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { DEFAULT_PROJECT_ID, type Project } from "@/types";
import { ProjectDetailPage } from "./ProjectDetailPage";
import { projectQueryKeys } from "./queryKeys";

const testProject: Project = {
  id: "project-1",
  name: "Default project",
  description: "Shared context",
  status: "active",
  context_summary: "Shared context",
  created_at: "2026-04-27T00:00:00Z",
  updated_at: "2026-04-27T00:00:00Z",
};

function renderPage(
  project: Project = testProject,
  initialPath = `/projects/${project.id}`,
) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(projectQueryKeys.detail(project.id), project);
  queryClient.setQueryData(["tasks", "project-members", project.id], {
    tasks: [],
    limit: 200,
    offset: 0,
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS} initialEntries={[initialPath]}>
        <Routes>
          <Route path="/projects/:projectId" element={<ProjectDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ProjectDetailPage", () => {
  it("presents settings, context, and linked work as distinct sections", () => {
    renderPage();

    expect(screen.getByRole("heading", { name: "Default project" })).toBeInTheDocument();
    expect(screen.getByRole("heading", { name: "Project settings" })).toBeInTheDocument();
    expect(screen.getByText(/Memory nodes/)).toBeInTheDocument();
    expect(screen.getByRole("link", { name: /Open context|Project context/ })).toHaveAttribute(
      "href",
      "/projects/project-1/context",
    );
    expect(screen.getByRole("heading", { name: /Linked tasks/ })).toBeInTheDocument();
  });

  it("shows delete project action for non-default projects", () => {
    renderPage();
    expect(screen.getByRole("button", { name: /^Delete project$/ })).toBeInTheDocument();
  });

  it("does not show delete project action for the built-in default project", () => {
    const builtIn: Project = {
      ...testProject,
      id: DEFAULT_PROJECT_ID,
      name: "Default project",
    };
    renderPage(builtIn, `/projects/${encodeURIComponent(DEFAULT_PROJECT_ID)}`);
    expect(screen.queryByRole("button", { name: /^Delete project$/ })).not.toBeInTheDocument();
  });
});
