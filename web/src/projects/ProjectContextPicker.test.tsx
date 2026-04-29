import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { ProjectContextPicker } from "./ProjectContextPicker";
import { projectQueryKeys } from "./queryKeys";

const projectId = "project-1";

const contextItems: ProjectContextItem[] = [
  {
    id: "ctx-risk",
    project_id: projectId,
    kind: "risk",
    title: "New One",
    body: "Risk details",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  },
  {
    id: "ctx-decision",
    project_id: projectId,
    kind: "decision",
    title: "Decision node",
    body: "Decision details",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  },
];

const contextEdges: ProjectContextEdge[] = [
  {
    id: "edge-1",
    project_id: projectId,
    source_context_id: "ctx-risk",
    target_context_id: "ctx-decision",
    relation: "depends_on",
    strength: 3,
    note: "",
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  },
];

function renderPicker(selectedIds: string[] = []) {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  });
  queryClient.setQueryData(projectQueryKeys.context(projectId), {
    items: contextItems,
    edges: contextEdges,
    limit: 100,
  });
  const onChange = vi.fn();

  function Wrapper({ children }: { children: ReactNode }) {
    return <QueryClientProvider client={queryClient}>{children}</QueryClientProvider>;
  }

  render(
    <ProjectContextPicker
      projectId={projectId}
      selectedIds={selectedIds}
      onChange={onChange}
    />,
    { wrapper: Wrapper },
  );

  return { onChange };
}

describe("ProjectContextPicker", () => {
  it("opens a searchable project-context list chooser and toggles selections", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker();

    await user.click(screen.getByRole("button", { name: /choose context/i }));
    const dialog = screen.getByRole("dialog", { name: /choose task context/i });

    await user.type(
      within(dialog).getByPlaceholderText(/search by title, body, or kind/i),
      "risk",
    );

    expect(within(dialog).getByText("New One")).toBeInTheDocument();
    expect(within(dialog).queryByText("Decision node")).not.toBeInTheDocument();

    await user.click(within(dialog).getByRole("checkbox", { name: /select new one/i }));

    expect(onChange).toHaveBeenCalledWith(["ctx-risk"]);
  });

  it("offers the same expandable tree view for choosing task context", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker();

    await user.click(screen.getByRole("button", { name: /choose context/i }));
    const dialog = screen.getByRole("dialog", { name: /choose task context/i });

    await user.click(within(dialog).getByRole("tab", { name: "Tree" }));

    expect(within(dialog).getByText(/depends on/i)).toBeInTheDocument();
    await user.click(
      within(dialog).getByRole("checkbox", { name: /select decision node/i }),
    );

    expect(onChange).toHaveBeenCalledWith(["ctx-decision"]);
  });
});
