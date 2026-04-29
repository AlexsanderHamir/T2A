import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { describe, expect, it, vi } from "vitest";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { ModalStackProvider } from "@/shared/ModalStackContext";
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
    return (
      <QueryClientProvider client={queryClient}>
        <ModalStackProvider>{children}</ModalStackProvider>
      </QueryClientProvider>
    );
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
  it("opens the choice dialog when a node is added from the chooser list", async () => {
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

    // The choice dialog opens before any onChange fires.
    expect(onChange).not.toHaveBeenCalled();
    const choice = screen.getByRole("dialog", {
      name: /reference project context/i,
    });
    await user.click(
      within(choice).getByTestId("project-context-choice-node-only"),
    );

    expect(onChange).toHaveBeenCalledWith(["ctx-risk"]);
  });

  it("expands node-with-children to add the descendant ids in BFS order", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker();

    await user.click(screen.getByRole("button", { name: /choose context/i }));
    const dialog = screen.getByRole("dialog", { name: /choose task context/i });

    await user.click(within(dialog).getByRole("checkbox", { name: /select new one/i }));

    const choice = screen.getByRole("dialog", {
      name: /reference project context/i,
    });
    await user.click(
      within(choice).getByTestId("project-context-choice-with-children"),
    );

    expect(onChange).toHaveBeenCalledWith(["ctx-risk", "ctx-decision"]);
  });

  it("removes a selected node from the summary chip list without prompting", async () => {
    const user = userEvent.setup();
    const { onChange } = renderPicker(["ctx-risk", "ctx-decision"]);

    expect(screen.getByText(/2 nodes selected/i)).toBeInTheDocument();
    await user.click(
      screen.getByRole("button", {
        name: /remove reference to new one/i,
      }),
    );
    expect(onChange).toHaveBeenCalledWith(["ctx-decision"]);
  });

  it("renders the selected items with title plus short id in the summary", () => {
    renderPicker(["ctx-risk"]);
    const chip = screen.getByText("New One").closest(".project-context-picker__chip");
    expect(chip).not.toBeNull();
    // The short-id helper strips dashes and lowercases the first six alnums.
    expect(chip?.textContent).toContain("ctxris");
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

    const choice = screen.getByRole("dialog", {
      name: /reference project context/i,
    });
    await user.click(
      within(choice).getByTestId("project-context-choice-node-only"),
    );

    expect(onChange).toHaveBeenCalledWith(["ctx-decision"]);
  });
});
