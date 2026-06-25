import { describe, expect, it } from "vitest";
import {
  deriveWorktreesPageMode,
  worktreesPageErrorMessage,
  worktreesPageTitle,
} from "./worktreesPageMode";

describe("deriveWorktreesPageMode", () => {
  it("returns loading while the query is pending", () => {
    expect(
      deriveWorktreesPageMode({
        isLoading: true,
        isError: false,
        repositoryCount: 0,
      }),
    ).toBe("loading");
  });

  it("returns error when fetch failed even with zero count", () => {
    expect(
      deriveWorktreesPageMode({
        isLoading: false,
        isError: true,
        repositoryCount: 0,
      }),
    ).toBe("error");
  });

  it("returns setup when loaded with no repositories", () => {
    expect(
      deriveWorktreesPageMode({
        isLoading: false,
        isError: false,
        repositoryCount: 0,
      }),
    ).toBe("setup");
  });

  it("returns manage when at least one repository exists", () => {
    expect(
      deriveWorktreesPageMode({
        isLoading: false,
        isError: false,
        repositoryCount: 2,
      }),
    ).toBe("manage");
  });
});

describe("worktreesPageErrorMessage", () => {
  it("uses default copy for unknown errors", () => {
    expect(worktreesPageErrorMessage(null)).toBe("Could not load repositories.");
  });

  it("explains Not Found separately from an empty repo list", () => {
    expect(worktreesPageErrorMessage(new Error("Not Found"))).toMatch(
      /git API may be unavailable/i,
    );
  });
});

describe("worktreesPageTitle", () => {
  it("uses Repositories for every page mode", () => {
    expect(worktreesPageTitle()).toBe("Repositories");
  });
});
