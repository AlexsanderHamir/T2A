/** Escape plain text for safe insertion into HTML. */
export function escapeHtml(s: string): string {
  return s
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
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
