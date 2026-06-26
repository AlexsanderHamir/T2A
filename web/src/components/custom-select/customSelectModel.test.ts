import { describe, expect, it } from "vitest";
import type { CustomSelectOption } from "./customSelectModel";
import {
  firstSelectableIndex,
  isCustomSelectHeader,
  lastSelectableIndex,
  nextSelectable,
  prevSelectable,
} from "./customSelectModel";

const h = (label: string): CustomSelectOption => ({
  type: "header",
  label,
});

const opt = (value: string, label = value): CustomSelectOption => ({
  value,
  label,
});

describe("isCustomSelectHeader", () => {
  it("narrows header options", () => {
    const o = h("Section");
    expect(isCustomSelectHeader(o)).toBe(true);
    if (isCustomSelectHeader(o)) {
      expect(o.label).toBe("Section");
    }
  });

  it("returns false for regular options", () => {
    expect(isCustomSelectHeader(opt("a", "A"))).toBe(false);
  });
});

describe("firstSelectableIndex", () => {
  it("skips a leading header", () => {
    expect(firstSelectableIndex([h("S"), opt("a")])).toBe(1);
  });

  it("returns 0 for a leading selectable or empty list", () => {
    expect(firstSelectableIndex([opt("a")])).toBe(0);
    expect(firstSelectableIndex([])).toBe(0);
    expect(firstSelectableIndex([h("only")])).toBe(0);
  });
});

describe("lastSelectableIndex", () => {
  it("finds the last non-header", () => {
    expect(lastSelectableIndex([opt("a"), h("S"), opt("b")])).toBe(2);
  });

  it("returns 0 when none or empty", () => {
    expect(lastSelectableIndex([opt("only")])).toBe(0);
    expect(lastSelectableIndex([])).toBe(0);
    expect(lastSelectableIndex([h("x")])).toBe(0);
  });
});

describe("nextSelectable", () => {
  const opts = [h("S"), opt("a"), opt("b")];

  it("moves forward past headers", () => {
    expect(nextSelectable(opts, 0)).toBe(1);
    expect(nextSelectable(opts, 1)).toBe(2);
  });

  it("wraps from last to first selectable", () => {
    expect(nextSelectable(opts, 2)).toBe(1);
  });

  it("returns from when only headers", () => {
    const headers = [h("a"), h("b")];
    expect(nextSelectable(headers, 0)).toBe(0);
  });
});

describe("prevSelectable", () => {
  const opts = [h("S"), opt("a"), opt("b")];

  it("moves backward to the previous selectable", () => {
    expect(prevSelectable(opts, 2)).toBe(1);
  });

  it("wraps from the first selectable to the last", () => {
    expect(prevSelectable(opts, 1)).toBe(2);
  });

  it("wraps from the leading selectable to the end", () => {
    expect(prevSelectable([opt("x"), h("S"), opt("y")], 0)).toBe(2);
  });

  it("returns from when only headers", () => {
    expect(prevSelectable([h("a")], 0)).toBe(0);
  });
});
