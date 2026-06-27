import { describe, expect, it } from "vitest";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
} from "./repositoryDisplay";

describe("repositoryDisplayName", () => {
  it("returns the last path segment", () => {
    expect(repositoryDisplayName("C:/Users/gomes/Documents/T2A")).toBe("T2A");
    expect(repositoryDisplayName("/repo/main")).toBe("main");
  });
});

describe("repositoryPathsEquivalent", () => {
  it("treats equivalent paths as equal regardless of separators", () => {
    expect(
      repositoryPathsEquivalent(
        "C:/Users/gomes/OneDrive/Documents/T2A",
        "C:\\Users\\gomes\\OneDrive\\Documents\\T2A",
      ),
    ).toBe(true);
  });
});
