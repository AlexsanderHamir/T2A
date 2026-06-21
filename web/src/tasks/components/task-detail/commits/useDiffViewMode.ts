import { useCallback, useState } from "react";
import type { ViewType } from "react-diff-view";

const storageKey = "hamix.commitDiff.viewMode";

function readStoredViewMode(): ViewType {
  try {
    const raw = localStorage.getItem(storageKey);
    if (raw === "split" || raw === "unified") {
      return raw;
    }
  } catch {
    /* private mode / disabled storage */
  }
  return "unified";
}

export function useDiffViewMode(): {
  viewMode: ViewType;
  setViewMode: (mode: ViewType) => void;
} {
  const [viewMode, setViewModeState] = useState<ViewType>(readStoredViewMode);

  const setViewMode = useCallback((mode: ViewType) => {
    setViewModeState(mode);
    try {
      localStorage.setItem(storageKey, mode);
    } catch {
      /* ignore */
    }
  }, []);

  return { viewMode, setViewMode };
}
