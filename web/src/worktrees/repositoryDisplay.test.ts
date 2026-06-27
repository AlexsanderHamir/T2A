import { describe, expect, it } from "vitest";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
  shouldShowWorktreePath,
  splitWorktreePath,
} from "./repositoryDisplay";

describe("repositoryDisplayName", () => {
  it("returns the last path segment", () => {
    expect(repositoryDisplayName("C:/Users/dev/Documents/hamix")).toBe("hamix");
    expect(repositoryDisplayName("/repo/main")).toBe("main");
  });
});

describe("repositoryPathsEquivalent", () => {
  it("treats equivalent paths as equal regardless of separators", () => {
    expect(
      repositoryPathsEquivalent(
        "C:/Users/dev/OneDrive/Documents/hamix",
        "C:\\Users\\dev\\OneDrive\\Documents\\hamix",
      ),
    ).toBe(true);
  });
});

describe("splitWorktreePath", () => {
  it("splits parent directory from the final segment", () => {
    expect(splitWorktreePath("C:\\Users\\dev\\Documents\\hamix")).toEqual({
      parent: "C:\\Users\\dev\\Documents\\",
      base: "hamix",
    });
    expect(splitWorktreePath("/repo/feature")).toEqual({
      parent: "/repo/",
      base: "feature",
    });
  });
});

describe("shouldShowWorktreePath", () => {
  it("hides path when it matches the repository header path", () => {
    expect(shouldShowWorktreePath("/repo/main", "/repo/main")).toBe(false);
    expect(shouldShowWorktreePath("/repo/feature", "/repo/main")).toBe(true);
  });
});
