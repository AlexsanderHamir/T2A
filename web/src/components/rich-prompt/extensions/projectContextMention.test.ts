import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { describe, expect, it } from "vitest";
import {
  ProjectContextMention,
  projectContextMentionLabel,
} from "./projectContextMention";

describe("projectContextMentionLabel", () => {
  it("formats id with short suffix and title", () => {
    expect(
      projectContextMentionLabel({
        id: "ctx-decision-7af2",
        title: "API plan",
      }),
    ).toBe("#API plan · ctxdec");
  });

  it("falls back to title only when the id is empty", () => {
    expect(projectContextMentionLabel({ id: "", title: "Constraint" })).toBe(
      "#Constraint",
    );
  });

  it("uses an explicit untitled label when title is blank", () => {
    expect(
      projectContextMentionLabel({ id: "ctx-1", title: "" }),
    ).toBe("#(untitled) · ctx1");
  });
});

describe("ProjectContextMention", () => {
  it("serializes to a chip span with stable data attributes", () => {
    const editor = new Editor({
      extensions: [StarterKit, ProjectContextMention],
      content: "<p></p>",
    });
    editor.commands.insertContentAt(1, [
      {
        type: "projectContextMention",
        attrs: { id: "ctx-decision-7af2", title: "API plan" },
      },
      { type: "text", text: " " },
    ]);
    const html = editor.getHTML();
    expect(html).toContain('data-project-context="true"');
    expect(html).toContain('data-project-context-id="ctx-decision-7af2"');
    expect(html).toContain('data-project-context-title="API plan"');
    expect(html).toContain("project-context-chip");
    expect(html).toContain("#API plan · ctxdec");
    editor.destroy();
  });

  it("round-trips back to a node when reading stored HTML", () => {
    const editor = new Editor({
      extensions: [StarterKit, ProjectContextMention],
      content:
        '<p><span data-project-context="true" data-project-context-id="ctx-1" data-project-context-title="Risk note">#Risk note · ctx1</span></p>',
    });
    let found = 0;
    editor.state.doc.descendants((node) => {
      if (node.type.name === "projectContextMention") {
        expect(node.attrs.id).toBe("ctx-1");
        expect(node.attrs.title).toBe("Risk note");
        found += 1;
      }
    });
    expect(found).toBe(1);
    editor.destroy();
  });
});
