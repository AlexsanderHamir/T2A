import { useEffect } from "react";

/** Matches `web/index.html` `<title>`; used as the app suffix for route-specific titles. */
export const DEFAULT_DOCUMENT_TITLE = "T2A — Tasks";

/**
 * Sets `document.title` for the current view (WCAG 2.4.2). Restores
 * {@link DEFAULT_DOCUMENT_TITLE} on unmount.
 */
export function useDocumentTitle(pageTitle: string | null | undefined): void {
  useEffect(() => {
    const trimmed =
      pageTitle === null || pageTitle === undefined ? "" : pageTitle.trim();
    document.title = trimmed
      ? `${trimmed} · ${DEFAULT_DOCUMENT_TITLE}`
      : DEFAULT_DOCUMENT_TITLE;
    return () => {
      document.title = DEFAULT_DOCUMENT_TITLE;
    };
  }, [pageTitle]);
}
