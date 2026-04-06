/** Escape plain text for safe insertion into HTML. */
export function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

function isSafeHref(rawHref: string): boolean {
  const href = rawHref.trim();
  if (!href) return false;
  if (href.startsWith("#")) return true;
  if (href.startsWith("/")) {
    // `//host` is protocol-relative (off-site), not a same-origin path.
    if (href.startsWith("//")) return false;
    return true;
  }
  return /^(https?:|mailto:)/i.test(href);
}

/** Roughly aligns with default API body size (~1 MiB UTF-8); avoids DOMParser on pathological prompts. */
const maxSanitizePromptCodeUnits = 350_000;

/**
 * Sanitize stored prompt HTML before injecting into the DOM.
 * Keeps a narrow rich-text allowlist and drops dangerous attributes/protocols.
 */
export function sanitizePromptHtml(input: string): string {
  if (!input.trim()) return "";
  if (input.length > maxSanitizePromptCodeUnits) {
    return `<p>${escapeHtml(input)}</p>`;
  }
  if (typeof DOMParser === "undefined") {
    return `<p>${escapeHtml(input)}</p>`;
  }

  const parser = new DOMParser();
  const doc = parser.parseFromString(input, "text/html");
  const allowedTags = new Set([
    "p",
    "br",
    "strong",
    "em",
    "u",
    "s",
    "code",
    "pre",
    "blockquote",
    "ul",
    "ol",
    "li",
    "a",
    "h1",
    "h2",
    "h3",
    "h4",
    "h5",
    "h6",
    "hr",
  ]);
  const dropWithContentTags = new Set([
    "script",
    "style",
    "iframe",
    "object",
    "embed",
    "link",
    "meta",
  ]);

  /** Caps recursion so adversarial deep nesting cannot blow the JS stack or freeze the tab. */
  const maxSanitizeDepth = 200;

  const sanitizeNode = (node: Node, depth: number): void => {
    if (node.nodeType === Node.TEXT_NODE) return;
    if (node.nodeType !== Node.ELEMENT_NODE) {
      node.parentNode?.removeChild(node);
      return;
    }
    if (depth > maxSanitizeDepth) {
      node.parentNode?.removeChild(node);
      return;
    }

    const el = node as HTMLElement;
    const tag = el.tagName.toLowerCase();

    if (!allowedTags.has(tag)) {
      const parent = el.parentNode;
      if (!parent) return;
      if (dropWithContentTags.has(tag)) {
        parent.removeChild(el);
        return;
      }
      // Unwrap: moved nodes must be sanitized (initial top-down walk can miss them).
      while (el.firstChild) {
        const ch = el.firstChild;
        parent.insertBefore(ch, el);
        sanitizeNode(ch, depth + 1);
      }
      parent.removeChild(el);
      return;
    }

    for (const attr of Array.from(el.attributes)) {
      const name = attr.name.toLowerCase();
      const value = attr.value;
      const allowedForA = tag === "a" && (name === "href" || name === "title");
      if (!allowedForA) {
        el.removeAttribute(attr.name);
        continue;
      }
      if (name === "href" && !isSafeHref(value)) {
        el.removeAttribute("href");
      }
    }

    if (tag === "a") {
      const href = el.getAttribute("href");
      if (href && /^https?:/i.test(href)) {
        el.setAttribute("target", "_blank");
        el.setAttribute("rel", "noopener noreferrer");
      }
    }

    const children = Array.from(el.childNodes);
    for (const child of children) sanitizeNode(child, depth + 1);
  };

  for (const child of Array.from(doc.body.childNodes)) sanitizeNode(child, 0);
  return doc.body.innerHTML;
}

/** Heuristic: stored prompt looks like TipTap HTML. */
export function looksLikeStoredHtml(s: string): boolean {
  const t = s.trim();
  return t.startsWith("<") && /<\/[a-z][\s\S]*>/i.test(t);
}

/** Migrate legacy plain-text prompts to minimal HTML paragraphs. */
export function plainTextToInitialHtml(plain: string): string {
  if (!plain.trim()) return "<p></p>";
  const parts = plain.split(/\n{2,}/);
  return parts
    .map((p) => `<p>${escapeHtml(p).replace(/\n/g, "<br>")}</p>`)
    .join("");
}

/** One-line preview for tables (strip tags from stored HTML). */
export function previewTextFromPrompt(s: string): string {
  if (!s.trim()) return "";
  if (!looksLikeStoredHtml(s)) return s.replace(/\s+/g, " ").trim();
  return s
    .replace(/<[^>]+>/g, " ")
    .replace(/\s+/g, " ")
    .trim();
}

/** True if the stored prompt has any visible text (after stripping markup). */
export function promptHasVisibleContent(s: string): boolean {
  return previewTextFromPrompt(s).length > 0;
}
