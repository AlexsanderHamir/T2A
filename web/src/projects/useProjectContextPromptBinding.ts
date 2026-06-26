import { useMemo } from "react";
import type { ProjectContextEdge, ProjectContextItem } from "@/types";
import { useProjectContext } from "./hooks";
import type { RichPromptEditorProjectContextProps } from "@/components/rich-prompt";

const EMPTY_CONTEXT_ITEMS: ProjectContextItem[] = [];
const EMPTY_CONTEXT_EDGES: ProjectContextEdge[] = [];

export type UseProjectContextPromptBindingOptions = {
  /**
   * Project the prompt should source `#` mention candidates from. When falsy,
   * the binding skips the network fetch and returns `null` so consumers can
   * render `RichPromptEditor` without project-context wiring.
   */
  projectId: string;
  selectedIds: string[];
  onSelectedIdsChange: (ids: string[]) => void;
};

/**
 * Compose the data inputs for `RichPromptEditor.projectContext` from the
 * shared `useProjectContext` query. Lives next to the data layer so any
 * tweak to the project-context fetch contract (limit, pinned filter, etc.)
 * lands here in one place rather than spreading through every prompt
 * surface that wants `#` mentions.
 *
 * Returns `null` when no project is selected so call sites can pass
 * `projectContext={binding ?? undefined}` without juggling sentinel values.
 */
export function useProjectContextPromptBinding(
  options: UseProjectContextPromptBindingOptions,
): RichPromptEditorProjectContextProps | null {
  const { projectId, selectedIds, onSelectedIdsChange } = options;
  const enabled = Boolean(projectId);
  const contextQuery = useProjectContext(projectId, {
    enabled,
    limit: 100,
    pinnedOnly: false,
  });

  return useMemo(() => {
    if (!enabled) return null;
    return {
      items: contextQuery.data?.items ?? EMPTY_CONTEXT_ITEMS,
      edges: contextQuery.data?.edges ?? EMPTY_CONTEXT_EDGES,
      selectedIds,
      onSelectedIdsChange,
    };
  }, [
    enabled,
    contextQuery.data?.items,
    contextQuery.data?.edges,
    selectedIds,
    onSelectedIdsChange,
  ]);
}
