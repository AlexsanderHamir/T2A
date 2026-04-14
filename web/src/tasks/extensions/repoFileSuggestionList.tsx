export type RepoSuggestionItem = { path: string };

type ListProps = {
  items: RepoSuggestionItem[];
  command: (item: RepoSuggestionItem) => void;
};

export function RepoFileSuggestionList({ items, command }: ListProps) {
  return (
    <div className="mention-dropdown tiptap-suggestion-list">
      <div className="tiptap-suggestion-list__head" aria-hidden="true">
        Repository files
      </div>
      <ul role="listbox" aria-label="Matching repository files">
        {items.length === 0 ? (
          <li className="tiptap-suggestion-list__empty" role="presentation">
            No matching files
          </li>
        ) : (
          items.map((item) => (
            <li key={item.path} className="mention-option" role="option">
              <button type="button" onClick={() => command(item)}>
                <span className="tiptap-suggestion-list__path">{item.path}</span>
              </button>
            </li>
          ))
        )}
      </ul>
    </div>
  );
}
