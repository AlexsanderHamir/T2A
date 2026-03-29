import { Extension } from "@tiptap/core";
import Suggestion, {
  exitSuggestion,
  type SuggestionKeyDownProps,
  type SuggestionProps,
} from "@tiptap/suggestion";
import { ReactRenderer } from "@tiptap/react";
import type { Instance as TippyInstance } from "tippy.js";
import tippy from "tippy.js";
import { PluginKey } from "@tiptap/pm/state";
import { searchRepoFiles } from "../../api";

export type RepoSuggestionItem = { path: string };

type ListProps = {
  items: RepoSuggestionItem[];
  command: (item: RepoSuggestionItem) => void;
};

function SuggestionList({ items, command }: ListProps) {
  return (
    <div className="mention-dropdown tiptap-suggestion-list">
      <ul role="listbox" aria-label="Matching files">
        {items.length === 0 ? (
          <li className="muted">No matching files</li>
        ) : (
          items.map((item) => (
            <li key={item.path} className="mention-option" role="option">
              <button type="button" onClick={() => command(item)}>
                {item.path}
              </button>
            </li>
          ))
        )}
      </ul>
    </div>
  );
}

export const repoFileSuggestionPluginKey = new PluginKey("repoFileSuggestion");

function clientRectOrFallback(
  fn: (() => DOMRect | null) | null | undefined,
): DOMRect {
  if (!fn) return new DOMRect(0, 0, 0, 0);
  return fn() ?? new DOMRect(0, 0, 0, 0);
}

export type RepoFilePickedPayload = {
  /** Path relative to REPO_ROOT (forward slashes). */
  path: string;
  /** Document position where the mention should be inserted (`@` was removed). */
  insertAt: number;
};

export type RepoFileSuggestionOptions = {
  onRepoUnavailable: () => void;
  onRepoAvailable: () => void;
  /** After the user picks a file, range UI runs; insert happens when they confirm. */
  onFilePicked?: (payload: RepoFilePickedPayload) => void;
};

export const RepoFileSuggestion = Extension.create<RepoFileSuggestionOptions>({
  name: "repoFileSuggestion",

  addOptions() {
    return {
      onRepoUnavailable: () => {},
      onRepoAvailable: () => {},
      onFilePicked: undefined as
        | ((payload: RepoFilePickedPayload) => void)
        | undefined,
    };
  },

  addProseMirrorPlugins() {
    const onUnavailable = this.options.onRepoUnavailable;
    const onAvailable = this.options.onRepoAvailable;
    const onFilePicked = this.options.onFilePicked;

    return [
      Suggestion<RepoSuggestionItem, RepoSuggestionItem>({
        pluginKey: repoFileSuggestionPluginKey,
        editor: this.editor,
        char: "@",
        allowSpaces: false,
        command: ({ editor, range, props }) => {
          const insertAt = range.from;
          const path = props.path.replace(/\\/g, "/");
          editor.chain().focus().deleteRange(range).run();
          exitSuggestion(editor.view, repoFileSuggestionPluginKey);
          onFilePicked?.({ path, insertAt });
        },
        items: async ({ query }) => {
          const paths = await searchRepoFiles(query);
          if (paths === null) {
            onUnavailable();
            return [];
          }
          onAvailable();
          return paths.map((path) => ({ path }));
        },
        render: () => {
          let component: ReactRenderer | null = null;
          let popup: TippyInstance | null = null;

          return {
            onStart(
              props: SuggestionProps<RepoSuggestionItem, RepoSuggestionItem>,
            ) {
              component = new ReactRenderer(SuggestionList, {
                props: {
                  items: props.items,
                  command: (item: RepoSuggestionItem) => {
                    props.command(item);
                  },
                },
                editor: props.editor,
              });

              if (!props.clientRect) {
                return;
              }

              const t = tippy(document.body, {
                getReferenceClientRect: () =>
                  clientRectOrFallback(props.clientRect ?? null),
                appendTo: () => document.body,
                content: component.element,
                showOnCreate: true,
                interactive: true,
                trigger: "manual",
                placement: "bottom-start",
              });
              popup = Array.isArray(t) ? t[0]! : t;
            },

            onUpdate(
              props: SuggestionProps<RepoSuggestionItem, RepoSuggestionItem>,
            ) {
              component?.updateProps({
                items: props.items,
                command: (item: RepoSuggestionItem) => {
                  props.command(item);
                },
              });
              popup?.setProps({
                getReferenceClientRect: () =>
                  clientRectOrFallback(props.clientRect ?? null),
              });
            },

            onKeyDown(props: SuggestionKeyDownProps) {
              if (props.event.key === "Escape") {
                popup?.hide();
                return true;
              }
              return false;
            },

            onExit() {
              popup?.destroy();
              component?.destroy();
            },
          };
        },
      }),
    ];
  },
});
