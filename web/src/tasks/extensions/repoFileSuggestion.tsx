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
import { RepoFileSuggestionList, type RepoSuggestionItem } from "./repoFileSuggestionList";
import {
  MENTION_POPOVER_Z_INDEX,
  referenceRectForSuggestion,
} from "./repoFileSuggestionReferenceRect";

export type { RepoSuggestionItem } from "./repoFileSuggestionList";

export const repoFileSuggestionPluginKey = new PluginKey("repoFileSuggestion");

export type RepoFilePickedPayload = {
  /** Path relative to the configured workspace repo (forward slashes). */
  path: string;
  /** Document position where the mention should be inserted (`@` was removed). */
  insertAt: number;
};

export type RepoFileSuggestionOptions = {
  onRepoUnavailable: () => void;
  onRepoAvailable: () => void;
  /**
   * Fires when TipTap starts or updates an @‑mention query (before the network request)
   * and again after the request finishes or the menu closes — for immediate “searching…” UX.
   */
  onSuggestFetchChange?: (busy: boolean) => void;
  /** After the user picks a file, range UI runs; insert happens when they confirm. */
  onFilePicked?: (payload: RepoFilePickedPayload) => void;
};

export const RepoFileSuggestion = Extension.create<RepoFileSuggestionOptions>({
  name: "repoFileSuggestion",

  addOptions() {
    return {
      onRepoUnavailable: () => {},
      onRepoAvailable: () => {},
      onSuggestFetchChange: undefined as
        | ((busy: boolean) => void)
        | undefined,
      onFilePicked: undefined as
        | ((payload: RepoFilePickedPayload) => void)
        | undefined,
    };
  },

  addProseMirrorPlugins() {
    const onUnavailable = this.options.onRepoUnavailable;
    const onAvailable = this.options.onRepoAvailable;
    const onSuggestFetchChange = this.options.onSuggestFetchChange;
    const onFilePicked = this.options.onFilePicked;
    const setFetchBusy = (busy: boolean) => {
      onSuggestFetchChange?.(busy);
    };

    // TipTap/ProseMirror may run overlapping async `view.update` passes; abort + returning []
    // lets a stale completion overwrite a newer successful `items` result and clears the menu.
    let mentionSearchSeq = 0;
    let lastRepoSuggestionItems: RepoSuggestionItem[] = [];

    return [
      Suggestion<RepoSuggestionItem, RepoSuggestionItem>({
        pluginKey: repoFileSuggestionPluginKey,
        editor: this.editor,
        char: "@",
        allowSpaces: false,
        // Default is only a regular space; allow @ after a newline inside the same block.
        allowedPrefixes: [" ", "\n"],
        command: ({ editor, range, props }) => {
          const insertAt = range.from;
          const path = props.path.replace(/\\/g, "/");
          editor.chain().focus().deleteRange(range).run();
          exitSuggestion(editor.view, repoFileSuggestionPluginKey);
          onFilePicked?.({ path, insertAt });
        },
        items: async ({ query }) => {
          mentionSearchSeq += 1;
          const seq = mentionSearchSeq;

          try {
            const paths = await searchRepoFiles(query);
            if (seq !== mentionSearchSeq) {
              return lastRepoSuggestionItems;
            }
            if (paths === null) {
              onUnavailable();
              lastRepoSuggestionItems = [];
              return [];
            }
            onAvailable();
            lastRepoSuggestionItems = paths.map((path) => ({ path }));
            return lastRepoSuggestionItems;
          } catch {
            if (seq !== mentionSearchSeq) {
              return lastRepoSuggestionItems;
            }
            // Transient errors: keep prior list if any; do not toggle the repo banner.
            return lastRepoSuggestionItems;
          } finally {
            // TipTap may interleave async view updates; always clear busy for this completion
            // so the inline hint never sticks if onStart/onUpdate ordering changes.
            if (seq === mentionSearchSeq) {
              setFetchBusy(false);
            }
          }
        },
        render: () => {
          let component: ReactRenderer | null = null;
          let popup: TippyInstance | null = null;
          let latestProps: SuggestionProps<
            RepoSuggestionItem,
            RepoSuggestionItem
          > | null = null;

          return {
            onBeforeStart() {
              setFetchBusy(true);
            },
            onBeforeUpdate() {
              setFetchBusy(true);
            },
            onStart(
              props: SuggestionProps<RepoSuggestionItem, RepoSuggestionItem>,
            ) {
              latestProps = props;
              component = new ReactRenderer(RepoFileSuggestionList, {
                props: {
                  items: props.items,
                  command: (item: RepoSuggestionItem) => {
                    props.command(item);
                  },
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
                /* Override tippy.css default dark #333 box + padding (see app-task-list-and-mentions.css) */
                theme: "repo-files-popover",
                arrow: false,
                maxWidth: "min(100vw - 2rem, 28rem)",
                offset: [0, 6],
              });
              popup = Array.isArray(t) ? t[0]! : t;
            },

            onUpdate(
              props: SuggestionProps<RepoSuggestionItem, RepoSuggestionItem>,
            ) {
              latestProps = props;
              component?.updateProps({
                items: props.items,
                command: (item: RepoSuggestionItem) => {
                  props.command(item);
                },
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
              lastRepoSuggestionItems = [];
              setFetchBusy(false);
              popup?.destroy();
              component?.destroy();
            },
          };
        },
      }),
    ];
  },
});
