import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it } from "vitest";
import { ROUTER_FUTURE_FLAGS } from "@/lib/routerFutureFlags";
import { DEFAULT_PROJECT_ID, type Project } from "@/types";
import { ProjectListPage } from "./ProjectListPage";
import { projectQueryKeys } from "./queryKeys";

function project(index: number, overrides: Partial<Project> = {}): Project {
  return {
    id: `project-${index}`,
    name: `Project ${index}`,
    description: `Context space ${index}`,
    status: "active",
    context_summary: "",
    created_at: "2026-04-27T00:00:00Z",
    updated_at: "2026-04-27T00:00:00Z",
    ...overrides,
  };
}

function renderPage(projects: Project[]) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(projectQueryKeys.list(true, 50), {
    projects,
    limit: 50,
  });

  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter future={ROUTER_FUTURE_FLAGS}>
        <ProjectListPage />
      </MemoryRouter>
    </QueryClientProvider>,
  );
}

describe("ProjectListPage", () => {
  it("renders a dense library for larger project collections", async () => {
    const projects = Array.from({ length: 10 }, (_, index) =>
      project(index + 1, index > 7 ? { status: "archived" } : {}),
    );

    renderPage(projects);

    const summary = screen.getByLabelText("Project summary");
    expect(within(summary).getByText("10")).toBeInTheDocument();
    expect(within(summary).getByText("8")).toBeInTheDocument();
    expect(within(summary).getByText("2")).toBeInTheDocument();

    const library = await screen.findByLabelText("Projects");
    expect(within(library).getAllByRole("link")).toHaveLength(10);
    expect(
      within(library).queryAllByRole("button", { name: /^Delete project / }),
    ).toHaveLength(0);
    expect(
      within(library).getByRole("link", { name: /^Open project Project 10$/ }),
    ).toHaveAttribute("href", "/projects/project-10");
  });

  it("does not surface row delete controls on the list", () => {
    const projects: Project[] = [
      project(0, { id: DEFAULT_PROJECT_ID, name: "Default project" }),
      project(1, { id: "custom-a", name: "Alpha" }),
      project(2, { id: "custom-b", name: "Beta" }),
    ];
    renderPage(projects);
    const library = screen.getByLabelText("Projects");
    expect(
      within(library).queryAllByRole("button", { name: /^Delete project / }),
    ).toHaveLength(0);
  });
});
