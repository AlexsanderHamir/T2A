import { projectContextShortId } from "@/projects/projectContextRefs";
import type { ProjectContextItem } from "@/types";

type Props = {
  /**
   * Items to render in display order. The parent passes the resolved items
   * already sorted to match the operator's selection order (see
   * `selectedProjectContextItems`).
   */
  items: ProjectContextItem[];
  disabled?: boolean;
  /** Optional remove handler; omitted when the form is read-only. */
  onRemove?: (id: string) => void;
};

/**
 * Read-only `REFERENCES` block that the prompt editor renders directly above
 * the editable content area whenever the task has selected project context
 * items. The block is purely a display affordance — it lives outside the
 * TipTap document so:
 *
 *   1. operators can never type into or partially delete it,
 *   2. the editor's `getHTML()` round-trip stays clean (no extra block tags
 *      sneaking into `initial_prompt`),
 *   3. backend prompt injection keeps reading `project_context_item_ids` as
 *      the canonical source of truth, never parsing this list out of HTML.
 *
 * The wrapper carries `data-project-references="true"` and each row has a
 * stable `data-project-context-id` attribute so any future tooling that
 * needs to introspect the rendered references can do so without fuzzy
 * text parsing.
 */
export function ProjectReferencesBlock({ items, disabled, onRemove }: Props) {
  if (items.length === 0) return null;
  return (
    <aside
      className="rich-prompt-references"
      data-project-references="true"
      aria-label="Selected project context (read-only)"
    >
      <header className="rich-prompt-references__header">
        <span className="rich-prompt-references__title">REFERENCES</span>
        <span className="rich-prompt-references__count muted">
          {items.length === 1 ? "1 node" : `${items.length} nodes`}
        </span>
      </header>
      <ul className="rich-prompt-references__list">
        {items.map((item) => {
          const shortId = projectContextShortId(item.id);
          return (
            <li
              key={item.id}
              className="rich-prompt-references__item"
              data-project-context-id={item.id}
            >
              <span className="rich-prompt-references__chip">
                <span className="rich-prompt-references__chip-title">
                  {item.title || "(untitled)"}
                </span>
                {shortId ? (
                  <span className="rich-prompt-references__chip-short-id muted">
                    · {shortId}
                  </span>
                ) : null}
                <span className="rich-prompt-references__chip-kind muted">
                  {item.kind}
                </span>
              </span>
              {onRemove ? (
                <button
                  type="button"
                  className="rich-prompt-references__remove"
                  onClick={() => onRemove(item.id)}
                  disabled={disabled}
                  aria-label={`Remove reference to ${item.title || "context node"}`}
                >
                  <svg
                    width="12"
                    height="12"
                    viewBox="0 0 12 12"
                    fill="none"
                    aria-hidden="true"
                  >
                    <path
                      d="M3 3l6 6M9 3l-6 6"
                      stroke="currentColor"
                      strokeWidth="1.4"
                      strokeLinecap="round"
                    />
                  </svg>
                </button>
              ) : null}
            </li>
          );
        })}
      </ul>
    </aside>
  );
}
