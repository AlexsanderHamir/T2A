import { describe, expect, it } from "vitest";
import {
  looksLikeStoredHtml,
  plainTextToInitialHtml,
  previewTextFromPrompt,
  promptHasVisibleContent,
  sanitizePromptHtml,
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

  it("sanitizes dangerous markup while preserving safe formatting", () => {
    const html = sanitizePromptHtml(
      '<p>Hello <strong>world</strong><script>alert(1)</script></p><a href="javascript:alert(1)" onclick="evil()">x</a>',
    );
    expect(html).toContain("<p>Hello <strong>world</strong></p>");
    expect(html).not.toContain("<script");
    expect(html).not.toContain("javascript:");
    expect(html).not.toContain("onclick=");
  });

  it("adds safe external link attributes and keeps relative links", () => {
    const html = sanitizePromptHtml(
      '<a href="https://example.com">external</a><a href="/tasks/1">internal</a>',
    );
    expect(html).toContain(
      '<a href="https://example.com" target="_blank" rel="noopener noreferrer">external</a>',
    );
    expect(html).toContain('<a href="/tasks/1">internal</a>');
  });

  it("strips protocol-relative hrefs that look like root-relative paths", () => {
    const html = sanitizePromptHtml(
      '<p><a href="//evil.example/phish">phish</a><a href="/safe">ok</a></p>',
    );
    expect(html).not.toContain("//evil.example");
    expect(html).not.toContain('href="//');
    expect(html).toContain('<a href="/safe">ok</a>');
  });

  it("drops entire dangerous tags instead of preserving their text", () => {
    const html = sanitizePromptHtml(
      '<p>before</p><script>alert(1)</script><style>.x{}</style><p>after</p>',
    );
    expect(html).toContain("<p>before</p>");
    expect(html).toContain("<p>after</p>");
    expect(html).not.toContain("alert(1)");
    expect(html).not.toContain(".x{}");
  });

  it("sanitizes children unwrapped from disallowed tags (e.g. svg)", () => {
    const html = sanitizePromptHtml(
      '<p>ok</p><svg><script>bad</script></svg><p>end</p>',
    );
    expect(html).toContain("<p>ok</p>");
    expect(html).toContain("<p>end</p>");
    expect(html).not.toContain("<script");
    expect(html).not.toContain("bad");
  });

  it("drops very deep markup without throwing", () => {
    const deep = `${"<div>".repeat(220)}<script>bad</script>${"</div>".repeat(220)}`;
    expect(() => sanitizePromptHtml(deep)).not.toThrow();
    const html = sanitizePromptHtml(deep);
    expect(html).not.toContain("<script");
    expect(html).not.toContain("bad");
  });
});
