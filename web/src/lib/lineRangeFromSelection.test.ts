import { describe, expect, it } from "vitest";
import { lineRangeFromSelection } from "./lineRangeFromSelection";

describe("lineRangeFromSelection", () => {
  it("returns null for empty selection", () => {
    expect(lineRangeFromSelection("a\nb", 1, 1)).toBeNull();
  });

  it("single line", () => {
    expect(lineRangeFromSelection("hello", 0, 5)).toEqual({
      startLine: 1,
      endLine: 1,
    });
  });

  it("two lines inclusive", () => {
    const t = "a\nb\nc";
    expect(lineRangeFromSelection(t, 0, 3)).toEqual({ startLine: 1, endLine: 2 });
  });

  it("selects middle line only", () => {
    const t = "a\nbb\nc";
    expect(lineRangeFromSelection(t, 2, 4)).toEqual({ startLine: 2, endLine: 2 });
  });
});
