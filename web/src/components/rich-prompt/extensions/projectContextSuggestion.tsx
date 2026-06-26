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
import {
  ProjectContextSuggestionList,
  type ProjectContextSuggestionItem,
} from "./projectContextSuggestionList";
import {
  MENTION_POPOVER_Z_INDEX,
  referenceRectForSuggestion,
} from "./repoFileSuggestionReferenceRect";
import type { ProjectContextItem } from "@/types";

export type { ProjectContextSuggestionItem } from "./projectContextSuggestionList";

export const projectContextSuggestionPluginKey = new PluginKey(
  "projectContextSuggestion",
);

export type ProjectContextPickedPayload = {
  item: ProjectContextItem;
  /**
   * Document position where the chip should be inserted (the `#` token has
   * already been removed from the document by the suggestion command).
   * Mirrors `RepoFilePickedPayload.insertAt` so the consumer can reuse the
   * same delayed-insert pattern (e.g. open a confirmation modal first, then
   * insert the chip when the user confirms).
   */
  insertAt: number;
};

export type ProjectContextSuggestionOptions = {
  /**
   * Returns the project context items the suggestion should search. Called
   * inline inside the items provider on every keystroke; consumers should
   * memoise the underlying list (e.g. via `useProjectContext` cache) so
   * each call is just a closure read.
   *
   * Returning `null` signals "no project selected" — the suggestion list
   * still opens but renders an explanatory empty state instead of swallowing
   * the trigger character silently.
   */
  getItems: () => ProjectContextItem[] | null;
  /**
   * Fires when the user picks a node. The consumer is responsible for the
   * actual chip insertion (after the optional node-only / with-children
   * confirmation). Mirrors `RepoFileSuggestionOptions.onFilePicked`.
   */
  onContextPicked?: (payload: ProjectContextPickedPayload) => void;
};

const MAX_SUGGESTION_RESULTS = 30;

function matchesQuery(item: ProjectContextItem, query: string): boolean {
  if (!query) return true;
  const q = query.toLowerCase();
  return (
    item.title.toLowerCase().includes(q) ||
    item.body.toLowerCase().includes(q) ||
    item.kind.toLowerCase().includes(q) ||
    item.id.toLowerCase().includes(q)
  );
}

export const ProjectContextSuggestion = Extension.create<ProjectContextSuggestionOptions>({
  name: "projectContextSuggestion",

  addOptions() {
    return {
      getItems: () => null,
      onContextPicked: undefined as
        | ((payload: ProjectContextPickedPayload) => void)
        | undefined,
    };
  },

  addProseMirrorPlugins() {
    const getItems = () => this.options.getItems();
    const onContextPicked = this.options.onContextPicked;

    return [
      Suggestion<ProjectContextSuggestionItem, ProjectContextSuggestionItem>({
        pluginKey: projectContextSuggestionPluginKey,
        editor: this.editor,
        char: "#",
        // Allow continued typing inside the trigger so titles with spaces
        // still match (e.g. `#API plan`). Matches the discoverability of the
        // existing `@` flow.
        allowSpaces: true,
        // Default is only a regular space; allow `#` after a newline inside
        // the same block so a fresh line in the prompt accepts the trigger.
        allowedPrefixes: [" ", "\n"],
        command: ({ editor, range, props }) => {
          const insertAt = range.from;
          editor.chain().focus().deleteRange(range).run();
          exitSuggestion(editor.view, projectContextSuggestionPluginKey);
          onContextPicked?.({ item: props.item, insertAt });
        },
        items: ({ query }) => {
          const items = getItems();
          if (!items) return [];
          const trimmed = query.trim();
          const filtered = items.filter((item) => matchesQuery(item, trimmed));
          return filtered
            .slice(0, MAX_SUGGESTION_RESULTS)
            .map((item) => ({ item }));
        },
        render: () => {
          let component: ReactRenderer | null = null;
          let popup: TippyInstance | null = null;
          let latestProps: SuggestionProps<
            ProjectContextSuggestionItem,
            ProjectContextSuggestionItem
          > | null = null;

          const emptyMessageFor = (
            props: SuggestionProps<
              ProjectContextSuggestionItem,
              ProjectContextSuggestionItem
            >,
          ): string => {
            const items = getItems();
            if (items === null) {
              return "Pick a project to enable #context references.";
            }
            if (items.length === 0) {
              return "This project has no context nodes yet.";
            }
            if (props.query.trim().length > 0) {
              return "No matching context nodes";
            }
            return "Type to search project context";
          };

          return {
            onStart(
              props: SuggestionProps<
                ProjectContextSuggestionItem,
                ProjectContextSuggestionItem
              >,
            ) {
              latestProps = props;
              component = new ReactRenderer(ProjectContextSuggestionList, {
                props: {
                  items: props.items,
                  command: (item: ProjectContextSuggestionItem) => {
                    props.command(item);
                  },
                  emptyMessage: emptyMessageFor(props),
                },
                editor: props.editor,
              });

              const t = tippy(document.body, {
                getReferenceClientRect: () =>
                  latestProps != null
                    ? referenceRectForSuggestion(latestProps)
                    : new DOMRect(0, 0, 0, 0),
                appendTo: () => document.body,
                content: component.element,
                showOnCreate: true,
                interactive: true,
                trigger: "manual",
                placement: "bottom-start",
                zIndex: MENTION_POPOVER_Z_INDEX,
                theme: "repo-files-popover",
                arrow: false,
                maxWidth: "min(100vw - 2rem, 32rem)",
                offset: [0, 6],
              });
              popup = Array.isArray(t) ? t[0]! : t;
            },

            onUpdate(
              props: SuggestionProps<
                ProjectContextSuggestionItem,
                ProjectContextSuggestionItem
              >,
            ) {
              latestProps = props;
              component?.updateProps({
                items: props.items,
                command: (item: ProjectContextSuggestionItem) => {
                  props.command(item);
                },
                emptyMessage: emptyMessageFor(props),
              });
              popup?.setProps({
                getReferenceClientRect: () =>
                  latestProps != null
                    ? referenceRectForSuggestion(latestProps)
                    : new DOMRect(0, 0, 0, 0),
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
