import { describe, expect, it } from "vitest";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
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
