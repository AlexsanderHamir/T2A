import { projectContextShortId } from "@/lib/projectContextRefs";
import type { ProjectContextItem } from "@/types";

export type ProjectContextSuggestionItem = {
  item: ProjectContextItem;
};

type ListProps = {
  items: ProjectContextSuggestionItem[];
  command: (item: ProjectContextSuggestionItem) => void;
  /** Status hint shown below the list when empty (e.g. no project selected). */
  emptyMessage?: string;
};

/**
 * Dropdown rendered by the `#` suggestion plugin. Mirrors the visual
 * structure of `RepoFileSuggestionList` so the editor reads as one
 * mention system with two triggers (`@` for repo files, `#` for project
 * memory). Each row shows the node title, kind tag, and short id so
 * operators can disambiguate same-titled nodes from different projects.
 */
export function ProjectContextSuggestionList({
  items,
  command,
  emptyMessage,
}: ListProps) {
  return (
    <div className="mention-dropdown tiptap-suggestion-list">
      <div className="tiptap-suggestion-list__head" aria-hidden="true">
        Project context
      </div>
      <ul role="listbox" aria-label="Matching project context nodes">
        {items.length === 0 ? (
          <li className="tiptap-suggestion-list__empty" role="presentation">
            {emptyMessage ?? "No matching context nodes"}
          </li>
        ) : (
          items.map(({ item }) => {
            const shortId = projectContextShortId(item.id);
            return (
              <li
                key={item.id}
                className="mention-option mention-option--project-context"
                role="option"
              >
                <button type="button" onClick={() => command({ item })}>
                  <span className="tiptap-suggestion-list__title">
                    {item.title || "(untitled)"}
                  </span>
                  <span className="tiptap-suggestion-list__meta">
                    <span className="tiptap-suggestion-list__kind">
                      {item.kind}
                    </span>
                    {shortId ? (
                      <span className="tiptap-suggestion-list__short-id">
                        · {shortId}
                      </span>
                    ) : null}
                  </span>
                </button>
              </li>
            );
          })
        )}
      </ul>
    </div>
  );
}
