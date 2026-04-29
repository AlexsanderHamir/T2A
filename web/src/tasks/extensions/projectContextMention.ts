import { mergeAttributes, Node } from "@tiptap/core";
import { projectContextShortId } from "@/projects/projectContextRefs";

/**
 * Render the chip text for an inserted project-context mention. Format
 * `#Title · shortId` matches the design in the plan; shortId is computed
 * from the full id by `projectContextShortId` so the displayed suffix is
 * always derivable, never stored separately on the node.
 */
export function projectContextMentionLabel(attrs: {
  id: string;
  title: string;
}): string {
  const id = (attrs.id ?? "").trim();
  const title = (attrs.title ?? "").trim() || "(untitled)";
  const shortId = projectContextShortId(id);
  if (!shortId) return `#${title}`;
  return `#${title} · ${shortId}`;
}

/**
 * Inline atom for `#project context` chips. Mirrors `RepoFileMention` so the
 * editor renders user-facing labels while keeping the canonical context item
 * id in `data-project-context-id`. The id is the source of truth for the
 * backend prompt-injection step (selected ids are passed via
 * `project_context_item_ids`, never parsed from the chip text).
 */
export const ProjectContextMention = Node.create({
  name: "projectContextMention",
  group: "inline",
  inline: true,
  atom: true,
  selectable: true,
  draggable: false,

  addAttributes() {
    return {
      id: {
        default: null as string | null,
        parseHTML: (el) => el.getAttribute("data-project-context-id"),
        renderHTML: (attrs) =>
          attrs.id != null
            ? { "data-project-context-id": attrs.id }
            : {},
      },
      title: {
        default: null as string | null,
        parseHTML: (el) => el.getAttribute("data-project-context-title"),
        renderHTML: (attrs) =>
          attrs.title != null
            ? { "data-project-context-title": attrs.title }
            : {},
      },
    };
  },

  parseHTML() {
    return [{ tag: 'span[data-project-context="true"]' }];
  },

  renderHTML({ node, HTMLAttributes }) {
    const id = node.attrs.id as string | null;
    const title = node.attrs.title as string | null;
    if (!id) {
      return [
        "span",
        mergeAttributes(HTMLAttributes, { class: "project-context-chip" }),
        "",
      ];
    }
    const text = projectContextMentionLabel({ id, title: title ?? "" });
    return [
      "span",
      mergeAttributes(HTMLAttributes, {
        "data-project-context": "true",
        class: "project-context-chip",
        title: text,
        "aria-label": `Project context reference: ${text}`,
      }),
      text,
    ];
  },

  renderText({ node }) {
    const id = node.attrs.id as string | null;
    if (!id) return "";
    const title = (node.attrs.title as string | null) ?? "";
    return projectContextMentionLabel({ id, title });
  },
});
