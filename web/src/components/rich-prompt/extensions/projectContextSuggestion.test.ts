import { Editor } from "@tiptap/core";
import Placeholder from "@tiptap/extension-placeholder";
import StarterKit from "@tiptap/starter-kit";
import { describe, expect, it, vi } from "vitest";
import type { ProjectContextItem } from "@/types";
import { ProjectContextMention } from "./projectContextMention";
import { ProjectContextSuggestion } from "./projectContextSuggestion";

const projectId = "project-1";

function makeItem(overrides: Partial<ProjectContextItem>): ProjectContextItem {
  return {
    id: overrides.id ?? "ctx-1",
    project_id: projectId,
    kind: overrides.kind ?? "decision",
    title: overrides.title ?? "Decision title",
    body: overrides.body ?? "Decision body",
    created_by: "user",
    pinned: false,
    created_at: "2026-04-29T00:00:00Z",
    updated_at: "2026-04-29T00:00:00Z",
    ...overrides,
  };
}

describe("ProjectContextSuggestion", () => {
  it("forwards every active project item to the dropdown when the trigger fires", () => {
    const items = [
      makeItem({ id: "ctx-decision", title: "API plan" }),
      makeItem({ id: "ctx-constraint", title: "Latency", kind: "constraint" }),
    ];
    const getItems = vi.fn(() => items);
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        ProjectContextMention,
        ProjectContextSuggestion.configure({
          getItems,
          onContextPicked: vi.fn(),
        }),
      ],
      content: "<p></p>",
    });
    editor.chain().insertContent("#").run();
    expect(getItems).toHaveBeenCalled();
    editor.destroy();
  });

  it("returns no items and does not throw when getItems is null", () => {
    const onContextPicked = vi.fn();
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        ProjectContextMention,
        ProjectContextSuggestion.configure({
          getItems: () => null,
          onContextPicked,
        }),
      ],
      content: "<p></p>",
    });
    expect(() => editor.chain().insertContent("#").run()).not.toThrow();
    expect(onContextPicked).not.toHaveBeenCalled();
    editor.destroy();
  });

  it("does not throw when the project has no context nodes yet", () => {
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        ProjectContextMention,
        ProjectContextSuggestion.configure({
          getItems: () => [],
        }),
      ],
      content: "<p></p>",
    });
    expect(() => editor.chain().insertContent("#").run()).not.toThrow();
    editor.destroy();
  });

  it("re-reads the items provider on each suggestion query (no stale closure)", () => {
    let pool: ProjectContextItem[] = [makeItem({ id: "ctx-1", title: "First" })];
    const getItems = vi.fn(() => pool);
    const editor = new Editor({
      extensions: [
        StarterKit,
        Placeholder.configure({ placeholder: "" }),
        ProjectContextMention,
        ProjectContextSuggestion.configure({
          getItems,
        }),
      ],
      content: "<p></p>",
    });
    editor.chain().insertContent("#").run();
    expect(getItems).toHaveBeenCalledTimes(1);

    // Simulate the underlying project query refreshing — the next trigger
    // should observe the new items via the same callback identity.
    pool = [makeItem({ id: "ctx-2", title: "Second" })];
    editor.chain().insertContent("a").run(); // updates the suggestion query
    expect(getItems.mock.calls.length).toBeGreaterThanOrEqual(2);
    expect(getItems.mock.results.at(-1)?.value?.[0]?.id).toBe("ctx-2");
    editor.destroy();
  });
});
