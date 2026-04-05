import { renderHook } from "@testing-library/react";
import { afterEach, describe, expect, it } from "vitest";
import {
  DEFAULT_DOCUMENT_TITLE,
  useDocumentTitle,
} from "./useDocumentTitle";

describe("useDocumentTitle", () => {
  afterEach(() => {
    document.title = DEFAULT_DOCUMENT_TITLE;
  });

  it("sets suffix title when page title is provided", () => {
    renderHook(() => useDocumentTitle("My task"));
    expect(document.title).toBe(`My task · ${DEFAULT_DOCUMENT_TITLE}`);
  });

  it("uses default title when page title is null, undefined, or blank", () => {
    const { rerender } = renderHook(
      (title: string | null | undefined) => useDocumentTitle(title),
      { initialProps: null as string | null | undefined },
    );
    expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
    rerender(undefined);
    expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
    rerender("   ");
    expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
  });

  it("restores default title on unmount", () => {
    const { unmount } = renderHook(() => useDocumentTitle("X"));
    expect(document.title).toBe(`X · ${DEFAULT_DOCUMENT_TITLE}`);
    unmount();
    expect(document.title).toBe(DEFAULT_DOCUMENT_TITLE);
  });
});
