import { describe, expect, it } from "vitest";
import {
  looksLikeStoredHtml,
  plainTextToInitialHtml,
  previewTextFromPrompt,
  promptHasVisibleContent,
} from "./promptFormat";

describe("promptFormat", () => {
  it("detects stored HTML", () => {
    expect(looksLikeStoredHtml("<p>x</p>")).toBe(true);
    expect(looksLikeStoredHtml("plain")).toBe(false);
  });

  it("escapes plain text to HTML paragraphs", () => {
    expect(plainTextToInitialHtml("a\n\nb")).toBe("<p>a</p><p>b</p>");
  });

  it("strips HTML for table preview", () => {
    expect(previewTextFromPrompt("<p>hello <strong>world</strong></p>")).toBe(
      "hello world",
    );
    expect(previewTextFromPrompt("plain")).toBe("plain");
  });

  it("detects visible prompt content", () => {
    expect(promptHasVisibleContent("")).toBe(false);
    expect(promptHasVisibleContent("   ")).toBe(false);
    expect(promptHasVisibleContent("<p></p>")).toBe(false);
    expect(promptHasVisibleContent("<p>hi</p>")).toBe(true);
  });
});
