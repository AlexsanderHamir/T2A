import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { ReactNode } from "react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { RichPromptEditor } from "./RichPromptEditor";

const projectId = "project-1";

const items: ProjectContextItem[] = [
  {
    id: "ctx-decision",
    project_id: projectId,
    kind: "decision",
    title: "API plan",
    body: "Use HTTP",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  },
  {
    id: "ctx-constraint",
    project_id: projectId,
    kind: "constraint",
    title: "Request budget",
    body: "Stay under 200ms",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  },
];

const edges: ProjectContextEdge[] = [
  {
    id: "edge-1",
    project_id: projectId,
    source_context_id: "ctx-decision",
    target_context_id: "ctx-constraint",
    relation: "supports",
    strength: 4,
    note: "",
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  },
];

function makeWrapper() {
  const queryClient = new QueryClient({
    defaultOptions: {
      queries: { retry: false, staleTime: Infinity },
      mutations: { retry: false },
    },
  });
  function Wrapper({ children }: { children: ReactNode }) {
    return (
      <QueryClientProvider client={queryClient}>
        <ModalStackProvider>{children}</ModalStackProvider>
      </QueryClientProvider>
    );
  }
  return { Wrapper };
}

describe("RichPromptEditor — project context wiring", () => {
  beforeEach(() => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ status: "ok", checks: {} }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );
  });

  afterEach(() => {
    vi.unstubAllGlobals();
  });

  it("does not render the REFERENCES block when no items are selected", () => {
    const { Wrapper } = makeWrapper();
    render(
      <RichPromptEditor
        id="rich-1"
        value="<p></p>"
        onChange={vi.fn()}
        projectContext={{
          items,
          edges,
          selectedIds: [],
          onSelectedIdsChange: vi.fn(),
        }}
      />,
      { wrapper: Wrapper },
    );
    expect(
      screen.queryByLabelText(/selected project context/i),
    ).not.toBeInTheDocument();
  });

  it("renders the read-only REFERENCES block above the editor with title + short id", () => {
    const { Wrapper } = makeWrapper();
    render(
      <RichPromptEditor
        id="rich-2"
        value="<p></p>"
        onChange={vi.fn()}
        projectContext={{
          items,
          edges,
          selectedIds: ["ctx-decision", "ctx-constraint"],
          onSelectedIdsChange: vi.fn(),
        }}
      />,
      { wrapper: Wrapper },
    );
    const block = screen.getByLabelText(/selected project context/i);
    expect(block).toHaveAttribute("data-project-references", "true");
    const apiPlanRow = within(block)
      .getByText("API plan")
      .closest("[data-project-context-id]");
    expect(apiPlanRow).not.toBeNull();
    expect(apiPlanRow).toHaveAttribute("data-project-context-id", "ctx-decision");
    // Short-id label includes ` · ctxdec` (lowercased, dashes stripped, 6 chars).
    expect(apiPlanRow?.textContent).toContain("ctxdec");
    // 2 nodes total -> "2 nodes" header label.
    expect(within(block).getByText(/2 nodes/i)).toBeInTheDocument();
  });

  it("calls onSelectedIdsChange when an item is removed via the REFERENCES block", async () => {
    const user = userEvent.setup();
    const { Wrapper } = makeWrapper();
    const onSelectedIdsChange = vi.fn();
    render(
      <RichPromptEditor
        id="rich-3"
        value="<p></p>"
        onChange={vi.fn()}
        projectContext={{
          items,
          edges,
          selectedIds: ["ctx-decision", "ctx-constraint"],
          onSelectedIdsChange,
        }}
      />,
      { wrapper: Wrapper },
    );
    await user.click(
      screen.getByRole("button", { name: /remove reference to api plan/i }),
    );
    expect(onSelectedIdsChange).toHaveBeenCalledWith(["ctx-constraint"]);
  });

  it("never renders the REFERENCES block when projectContext is omitted", () => {
    const { Wrapper } = makeWrapper();
    render(
      <RichPromptEditor
        id="rich-4"
        value="<p></p>"
        onChange={vi.fn()}
      />,
      { wrapper: Wrapper },
    );
    expect(
      screen.queryByLabelText(/selected project context/i),
    ).not.toBeInTheDocument();
  });
});
