import type { SuggestionProps } from "@tiptap/suggestion";
import type { RepoSuggestionItem } from "./repoFileSuggestionList";

/** Keep in sync with `--z-portal-popover` / `--z-mention-popover` in app-design-tokens.css */
export const MENTION_POPOVER_Z_INDEX = 13000;

/** TipTap may pass a clientRect that returns null or 0×0 before the suggestion decoration is in the DOM — fall back to coords at the match range. */
export function referenceRectForSuggestion(
  props: SuggestionProps<RepoSuggestionItem, RepoSuggestionItem>,
): DOMRect {
  const r = props.clientRect?.() ?? null;
  if (r && (r.width > 0 || r.height > 0)) {
    return r;
  }
  try {
    const coords = props.editor.view.coordsAtPos(props.range.from);
    return new DOMRect(
      coords.left,
      coords.top,
      coords.right - coords.left,
      coords.bottom - coords.top,
    );
  } catch {
    return new DOMRect(0, 0, 0, 0);
  }
}
