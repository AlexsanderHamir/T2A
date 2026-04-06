import Prism from "prismjs";
import "prismjs/components/prism-c";
import "prismjs/components/prism-cpp";
import "prismjs/components/prism-csharp";
import "prismjs/components/prism-bash";
import "prismjs/components/prism-diff";
import "prismjs/components/prism-docker";
import "prismjs/components/prism-go";
import "prismjs/components/prism-git";
import "prismjs/components/prism-ini";
import "prismjs/components/prism-java";
import "prismjs/components/prism-json";
import "prismjs/components/prism-jsx";
import "prismjs/components/prism-markdown";
import "prismjs/components/prism-python";
import "prismjs/components/prism-ruby";
import "prismjs/components/prism-rust";
import "prismjs/components/prism-sql";
import "prismjs/components/prism-toml";
import "prismjs/components/prism-tsx";
import "prismjs/components/prism-typescript";
import "prismjs/components/prism-yaml";

function escapePreviewHtml(raw: string): string {
  return raw
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/** Avoid splitting a UTF-16 surrogate pair when slicing for escape caps. */
function truncateUtf16Safe(s: string, max: number): string {
  if (s.length <= max) {
    return s;
  }
  if (max <= 0) {
    return "";
  }
  let t = s.slice(0, max);
  const last = t.charCodeAt(t.length - 1);
  if (last >= 0xd800 && last <= 0xdbff) {
    t = t.slice(0, -1);
  }
  return t;
}

/** Prism on multi‑MB strings can freeze the tab; repo preview may be up to ~32 MiB. */
const maxPrismHighlightChars = 1_000_000;

/** HTML escape still scans the full string several times; cap before escape on huge previews. */
const maxEscapedPreviewChars = 4_000_000;

export function highlightPreviewContent(
  content: string,
  prismLanguage: string,
): string {
  if (content.length > maxPrismHighlightChars) {
    const toEscape =
      content.length > maxEscapedPreviewChars
        ? truncateUtf16Safe(content, maxEscapedPreviewChars)
        : content;
    return escapePreviewHtml(toEscape);
  }
  const grammar = Prism.languages[prismLanguage];
  if (!grammar) return escapePreviewHtml(content);
  return Prism.highlight(content, grammar, prismLanguage);
}
