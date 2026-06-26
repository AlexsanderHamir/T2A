import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { ModalStackProvider } from "@/shared/ModalStackContext";
import { ProjectContextChoiceDialog } from "./ProjectContextChoiceDialog";

const projectId = "project-1";

function makeItem(overrides: Partial<ProjectContextItem>): ProjectContextItem {
  return {
    id: overrides.id ?? "ctx-root",
    project_id: projectId,
    kind: overrides.kind ?? "decision",
    title: overrides.title ?? "Root memory",
    body: overrides.body ?? "Root body",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
    ...overrides,
  };
}

function edge(source: string, target: string, id = `${source}->${target}`): ProjectContextEdge {
  return {
    id,
    project_id: projectId,
    source_context_id: source,
    target_context_id: target,
    relation: "supports",
    strength: 3,
    note: "",
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
  };
}

describe("ProjectContextChoiceDialog", () => {
  it("calls onConfirm with nodeOnly when the operator picks the single-node option", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <ModalStackProvider>
        <ProjectContextChoiceDialog
          item={makeItem({})}
          edges={[edge("ctx-root", "ctx-child")]}
          selectedIds={[]}
          onClose={vi.fn()}
          onConfirm={onConfirm}
        />
      </ModalStackProvider>,
    );
    await user.click(screen.getByTestId("project-context-choice-node-only"));
    expect(onConfirm).toHaveBeenCalledWith("nodeOnly");
  });

  it("calls onConfirm with withChildren and previews the descendant count", async () => {
    const user = userEvent.setup();
    const onConfirm = vi.fn();
    render(
      <ModalStackProvider>
        <ProjectContextChoiceDialog
          item={makeItem({})}
          edges={[
            edge("ctx-root", "ctx-child"),
            edge("ctx-child", "ctx-grandchild"),
          ]}
          selectedIds={[]}
          onClose={vi.fn()}
          onConfirm={onConfirm}
        />
      </ModalStackProvider>,
    );
    expect(
      screen.getByText(/3 references total/i),
    ).toBeInTheDocument();
    await user.click(
      screen.getByTestId("project-context-choice-with-children"),
    );
    expect(onConfirm).toHaveBeenCalledWith("withChildren");
  });

  it("disables the children option when the picked node has no outgoing edges", () => {
    render(
      <ModalStackProvider>
        <ProjectContextChoiceDialog
          item={makeItem({})}
          edges={[edge("ctx-other", "ctx-leaf")]}
          selectedIds={[]}
          onClose={vi.fn()}
          onConfirm={vi.fn()}
        />
      </ModalStackProvider>,
    );
    expect(
      screen.getByTestId("project-context-choice-with-children"),
    ).toBeDisabled();
    expect(screen.getByText(/no outgoing connections yet/i)).toBeInTheDocument();
  });

  it("calls onClose when the operator cancels", async () => {
    const user = userEvent.setup();
    const onClose = vi.fn();
    render(
      <ModalStackProvider>
        <ProjectContextChoiceDialog
          item={makeItem({})}
          edges={[]}
          selectedIds={[]}
          onClose={onClose}
          onConfirm={vi.fn()}
        />
      </ModalStackProvider>,
    );
    await user.click(screen.getByTestId("project-context-choice-cancel"));
    expect(onClose).toHaveBeenCalled();
  });
});
