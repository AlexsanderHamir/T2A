import { describe, expect, it } from "vitest";
import { highlightPreviewContent } from "./filePreviewHighlight";

describe("highlightPreviewContent", () => {
  it("escapes raw HTML when language grammar is unavailable", () => {
    const src = '<img src=x onerror="alert(1)">plain';
    expect(highlightPreviewContent(src, "unknown-language")).toBe(
      "&lt;img src=x onerror=&quot;alert(1)&quot;&gt;plain",
    );
  });

  it("returns highlighted markup when language grammar exists", () => {
    const out = highlightPreviewContent("const a = 1", "typescript");
    expect(out).toContain("<span");
  });

  it("skips Prism when content exceeds size cap", () => {
    const big = "a".repeat(1_000_001);
    const out = highlightPreviewContent(big, "typescript");
    expect(out).not.toContain("<span");
    expect(out).toBe(big);
  });
});
