import { mergeAttributes, Node } from "@tiptap/core";

export function repoFileMentionLabel(attrs: {
  path: string;
  lineStart?: number | null;
  lineEnd?: number | null;
}): string {
  const { path, lineStart, lineEnd } = attrs;
  if (
    lineStart != null &&
    lineEnd != null &&
    Number.isFinite(lineStart) &&
    Number.isFinite(lineEnd)
  ) {
    return `@${path}(${lineStart}-${lineEnd})`;
  }
  return `@${path}`;
}

/** Inline atom for @repo/path and @repo/path(1-10) so mentions render as chips in the editor. */
export const RepoFileMention = Node.create({
  name: "repoFileMention",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,
  draggable: false,

  addAttributes() {
    return {
      path: {
        default: null as string | null,
        parseHTML: (el) => el.getAttribute("data-path"),
        renderHTML: (attrs) =>
          attrs.path != null ? { "data-path": attrs.path } : {},
      },
      lineStart: {
        default: null as number | null,
        parseHTML: (el) => {
          const v = el.getAttribute("data-line-start");
          if (v == null || v === "") return null;
          const n = parseInt(v, 10);
          return Number.isFinite(n) ? n : null;
        },
        renderHTML: (attrs) =>
          attrs.lineStart != null
            ? { "data-line-start": String(attrs.lineStart) }
            : {},
      },
      lineEnd: {
        default: null as number | null,
        parseHTML: (el) => {
          const v = el.getAttribute("data-line-end");
          if (v == null || v === "") return null;
          const n = parseInt(v, 10);
          return Number.isFinite(n) ? n : null;
        },
        renderHTML: (attrs) =>
          attrs.lineEnd != null ? { "data-line-end": String(attrs.lineEnd) } : {},
      },
    };
  },

  parseHTML() {
    return [{ tag: 'span[data-repo-file="true"]' }];
  },

  renderHTML({ node, HTMLAttributes }) {
    const path = node.attrs.path as string | null;
    if (!path) {
      return ["span", mergeAttributes(HTMLAttributes, { class: "repo-file-chip" }), ""];
    }
    const lineStart = node.attrs.lineStart as number | null;
    const lineEnd = node.attrs.lineEnd as number | null;
    const text = repoFileMentionLabel({ path, lineStart, lineEnd });
    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-repo-file": "true",
        class: "repo-file-chip",
        title: text,
        "aria-label": `File reference: ${text}`,
      }),
      text,
    ];
  },

  renderText({ node }) {
    const path = node.attrs.path as string | null;
    if (!path) return "";
    const lineStart = node.attrs.lineStart as number | null;
    const lineEnd = node.attrs.lineEnd as number | null;
    return repoFileMentionLabel({ path, lineStart, lineEnd });
  },
});
