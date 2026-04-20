import { useCallback, useMemo, useState } from "react";

/**
 * useTaskListSelection — section-local selection state for the
 * bulk-action experience on the task list (Stage 5 of task
 * scheduling).
 *
 * Why local rather than URL-synced: bulk selection is ephemeral
 * by design (per the plan's "Selection state clears on filter
 * change, sort change, or successful bulk action — preventing the
 * classic 'I selected 12, applied filter, now Apply to selection
 * targets things I cant see'" guard). Lifting to URL would make
 * stale selections survivable across reloads/back-button, which
 * is the opposite of what we want.
 *
 * The hook accepts the *currently visible* row id list and uses
 * it for two things:
 *   1. `selectedVisibleIds` — the intersection of the running
 *      selection with what's actually on screen (callers operate
 *      on this when firing the bulk PATCH; never on the raw set).
 *   2. `allVisibleSelected` / `someVisibleSelected` — for the
 *      header tri-state checkbox.
 *
 * `clearSelection` is exposed so callers can wipe state on
 * filter/sort change or after a successful bulk operation.
 */
export type TaskListSelection = ReturnType<typeof useTaskListSelection>;

export function useTaskListSelection(visibleIds: ReadonlyArray<string>) {
  const [selectedIds, setSelectedIds] = useState<ReadonlySet<string>>(
    () => new Set<string>(),
  );

  const visibleSet = useMemo(() => new Set(visibleIds), [visibleIds]);

  const selectedVisibleIds = useMemo(
    () => visibleIds.filter((id) => selectedIds.has(id)),
    [visibleIds, selectedIds],
  );

  const allVisibleSelected =
    visibleIds.length > 0 && selectedVisibleIds.length === visibleIds.length;
  const someVisibleSelected =
    selectedVisibleIds.length > 0 && !allVisibleSelected;

  const toggle = useCallback((id: string) => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      return next;
    });
  }, []);

  const setRowSelected = useCallback((id: string, selected: boolean) => {
    setSelectedIds((prev) => {
      if (selected) {
        if (prev.has(id)) return prev;
        const next = new Set(prev);
        next.add(id);
        return next;
      }
      if (!prev.has(id)) return prev;
      const next = new Set(prev);
      next.delete(id);
      return next;
    });
  }, []);

  const selectAllVisible = useCallback(() => {
    setSelectedIds((prev) => {
      const next = new Set(prev);
      for (const id of visibleIds) next.add(id);
      return next;
    });
  }, [visibleIds]);

  const deselectAllVisible = useCallback(() => {
    setSelectedIds((prev) => {
      let changed = false;
      const next = new Set(prev);
      for (const id of visibleIds) {
        if (next.delete(id)) changed = true;
      }
      return changed ? next : prev;
    });
  }, [visibleIds]);

  const toggleAllVisible = useCallback(() => {
    if (allVisibleSelected) deselectAllVisible();
    else selectAllVisible();
  }, [allVisibleSelected, deselectAllVisible, selectAllVisible]);

  const clearSelection = useCallback(() => {
    setSelectedIds((prev) => (prev.size === 0 ? prev : new Set<string>()));
  }, []);

  return {
    selectedIds,
    selectedVisibleIds,
    allVisibleSelected,
    someVisibleSelected,
    isSelected: useCallback((id: string) => selectedIds.has(id), [selectedIds]),
    isVisible: useCallback((id: string) => visibleSet.has(id), [visibleSet]),
    toggle,
    setRowSelected,
    selectAllVisible,
    deselectAllVisible,
    toggleAllVisible,
    clearSelection,
  } as const;
}
