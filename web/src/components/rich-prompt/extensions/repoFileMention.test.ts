import { Editor } from "@tiptap/core";
import StarterKit from "@tiptap/starter-kit";
import { describe, expect, it } from "vitest";
import { RepoFileMention, repoFileMentionLabel } from "./repoFileMention";

describe("repoFileMentionLabel", () => {
  it("formats path only", () => {
    expect(repoFileMentionLabel({ path: "engine/benchmark/discovery_test.go" })).toBe(
      "@engine/benchmark/discovery_test.go",
    );
  });

  it("formats path with line range", () => {
    expect(
      repoFileMentionLabel({
        path: "engine/benchmark/discovery_test.go",
        lineStart: 2,
        lineEnd: 10,
      }),
    ).toBe("@engine/benchmark/discovery_test.go(2-10)");
  });
});

describe("RepoFileMention", () => {
  it("serializes to a chip span with data attributes", () => {
    const editor = new Editor({
      extensions: [StarterKit, RepoFileMention],
      content: "<p></p>",
    });
    editor.commands.insertContentAt(1, [
      {
        type: "repoFileMention",
        attrs: { path: "AGENTS.md", lineStart: 1, lineEnd: 5 },
      },
      { type: "text", text: " " },
    ]);
    const html = editor.getHTML();
    expect(html).toContain('data-repo-file="true"');
    expect(html).toContain('data-path="AGENTS.md"');
    expect(html).toContain('data-line-start="1"');
    expect(html).toContain('data-line-end="5"');
    expect(html).toContain("repo-file-chip");
    expect(html).toContain("@AGENTS.md(1-5)");
    editor.destroy();
  });
});
