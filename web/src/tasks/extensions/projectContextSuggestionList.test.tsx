import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { ProjectContextItem } from "@/types";
import { ProjectContextSuggestionList } from "./projectContextSuggestionList";

function makeItem(overrides: Partial<ProjectContextItem> = {}): ProjectContextItem {
  return {
    id: overrides.id ?? "ctx-1",
    project_id: "project-1",
    kind: overrides.kind ?? "decision",
    title: overrides.title ?? "Decision",
    body: "",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
    ...overrides,
  };
}

describe("ProjectContextSuggestionList", () => {
  it("renders title, kind, and short id for each item", () => {
    const items = [
      { item: makeItem({ id: "ctx-decision", title: "Use HTTP" }) },
      { item: makeItem({ id: "ctx-constraint-2026", title: "Latency", kind: "constraint" }) },
    ];
    render(
      <ProjectContextSuggestionList items={items} command={vi.fn()} />,
    );
    expect(screen.getByText("Use HTTP")).toBeInTheDocument();
    expect(screen.getByText("Latency")).toBeInTheDocument();
    expect(screen.getByText("decision")).toBeInTheDocument();
    expect(screen.getByText("constraint")).toBeInTheDocument();
    // Short id is the first six alphanumeric characters lowercased.
    expect(screen.getByText("· ctxdec")).toBeInTheDocument();
    expect(screen.getByText("· ctxcon")).toBeInTheDocument();
  });

  it("invokes command when an item is selected", async () => {
    const user = userEvent.setup();
    const command = vi.fn();
    const item = makeItem({ id: "ctx-1", title: "Pick me" });
    render(
      <ProjectContextSuggestionList items={[{ item }]} command={command} />,
    );
    await user.click(screen.getByRole("button", { name: /pick me/i }));
    expect(command).toHaveBeenCalledWith({ item });
  });

  it("falls back to the supplied empty message when the list is empty", () => {
    render(
      <ProjectContextSuggestionList
        items={[]}
        command={vi.fn()}
        emptyMessage="Pick a project to enable #context references."
      />,
    );
    expect(
      screen.getByText(/pick a project to enable #context references/i),
    ).toBeInTheDocument();
  });
});
