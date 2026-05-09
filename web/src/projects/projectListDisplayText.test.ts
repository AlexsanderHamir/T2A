import { describe, expect, it } from "vitest";
import {
  PROJECT_LIST_TITLE_MAX,
  truncateGraphDescription,
  truncateListDescription,
  truncateListTitle,
} from "./projectListDisplayText";

describe("truncateListTitle", () => {
  it("returns short strings unchanged", () => {
    expect(truncateListTitle("Auth refactor")).toBe("Auth refactor");
  });

  it("truncates with ellipsis when over limit", () => {
    const long = "a".repeat(PROJECT_LIST_TITLE_MAX + 10);
    const out = truncateListTitle(long);
    expect(out.endsWith("…")).toBe(true);
    expect(Array.from(out).length).toBeLessThanOrEqual(PROJECT_LIST_TITLE_MAX + 1);
  });

  it("truncates on extended grapheme count, not UTF-16 units", () => {
    const s = "🔒".repeat(80);
    const out = truncateListTitle(s);
    expect(out.endsWith("…")).toBe(true);
    expect(Array.from(out).length).toBeLessThanOrEqual(PROJECT_LIST_TITLE_MAX + 1);
  });
});

describe("truncateListDescription", () => {
  it("caps long descriptions", () => {
    const d = "word ".repeat(80);
    expect(truncateListDescription(d).length).toBeLessThan(d.length);
  });
});

describe("truncateGraphDescription", () => {
  it("uses a shorter cap than list descriptions", () => {
    const s = "x".repeat(200);
    expect(truncateGraphDescription(s).length).toBeLessThan(truncateListDescription(s).length);
  });
});
