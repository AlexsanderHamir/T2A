import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { ProjectContextTreeView } from "./ProjectContextTreeView";

const baseItem = {
  project_id: "project-1",
  kind: "note",
  body: "",
  created_by: "user",
  pinned: false,
  created_at: "2026-04-27T00:00:00Z",
  updated_at: "2026-04-27T00:00:00Z",
} satisfies Omit<ProjectContextItem, "id" | "title">;

function item(id: string, title: string): ProjectContextItem {
  return {
    ...baseItem,
    id,
    title,
  };
}

function edge(
  id: string,
  source_context_id: string,
  target_context_id: string,
): ProjectContextEdge {
  return {
    id,
    project_id: "project-1",
    source_context_id,
    target_context_id,
    relation: "depends_on",
    strength: 1,
    note: "",
    created_at: "2026-04-27T00:00:00Z",
    updated_at: "2026-04-27T00:00:00Z",
  };
}

describe("ProjectContextTreeView", () => {
  it("shows the top ancestor collapsed before showing descendants", () => {
    const root = item("root", "Root memory");
    const child = item("child", "Child memory");

    const { container } = render(
      <ProjectContextTreeView
        items={[child, root]}
        edges={[edge("edge-1", root.id, child.id)]}
      />,
    );

    const tree = container.querySelector(".project-context-tree");
    expect(tree?.querySelector("summary strong")).toHaveTextContent("Root memory");
    expect(tree).not.toHaveAttribute("open");
    for (const childText of screen.getAllByText("Child memory")) {
      expect(childText).not.toBeVisible();
    }
  });
});
