import { describe, expect, it } from "vitest";
import { decideCreateEntry } from "./decideCreateEntry";

describe("decideCreateEntry", () => {
  it("shows picker while draft list is loading", () => {
    expect(
      decideCreateEntry({
        isPending: true,
        isError: false,
        errorMessage: null,
        draftCount: 0,
      }),
    ).toEqual({ kind: "showPicker" });
  });

  it("opens fresh form with hint when draft list fails", () => {
    expect(
      decideCreateEntry({
        isPending: false,
        isError: true,
        errorMessage: "network",
        draftCount: 0,
      }),
    ).toEqual({ kind: "openFreshForm", entryDraftErrorHint: "network" });
  });

  it("shows picker when drafts exist", () => {
    expect(
      decideCreateEntry({
        isPending: false,
        isError: false,
        errorMessage: null,
        draftCount: 2,
      }),
    ).toEqual({ kind: "showPicker" });
  });

  it("opens fresh form when no drafts", () => {
    expect(
      decideCreateEntry({
        isPending: false,
        isError: false,
        errorMessage: null,
        draftCount: 0,
      }),
    ).toEqual({ kind: "openFreshForm", entryDraftErrorHint: null });
  });
});
