import { describe, expect, it } from "vitest";
import { highlightPreviewContent } from "./filePreviewHighlight";

describe("highlightPreviewContent", () => {
  it("returns input text when language grammar is unavailable", () => {
    const src = "plain text";
    expect(highlightPreviewContent(src, "unknown-language")).toBe(src);
  });

  it("returns highlighted markup when language grammar exists", () => {
    const out = highlightPreviewContent("const a = 1", "typescript");
    expect(out).toContain("<span");
  });
});
