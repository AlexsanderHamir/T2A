import { describe, expect, it } from "vitest";
import {
  registerWorktreePathDisabled,
  registerWorktreePathPlaceholder,
} from "./registerWorktreePathSelect";

describe("registerWorktreePathPlaceholder", () => {
  it("returns loading copy while fetching", () => {
    expect(
      registerWorktreePathPlaceholder({ loading: true, optionCount: 0 }),
    ).toBe("Loading linked worktrees…");
    expect(
      registerWorktreePathPlaceholder({ loading: true, optionCount: 3 }),
    ).toBe("Loading linked worktrees…");
  });

  it("returns empty copy when settled with no options", () => {
    expect(
      registerWorktreePathPlaceholder({ loading: false, optionCount: 0 }),
    ).toBe("No unregistered linked worktrees for this repository.");
  });

  it("returns prompt when options are available", () => {
    expect(
      registerWorktreePathPlaceholder({ loading: false, optionCount: 2 }),
    ).toBe("Select a linked worktree");
  });
});

describe("registerWorktreePathDisabled", () => {
  it("is disabled while pending, loading, or empty", () => {
    expect(
      registerWorktreePathDisabled({ pending: true, loading: false, optionCount: 2 }),
    ).toBe(true);
    expect(
      registerWorktreePathDisabled({ pending: false, loading: true, optionCount: 2 }),
    ).toBe(true);
    expect(
      registerWorktreePathDisabled({ pending: false, loading: false, optionCount: 0 }),
    ).toBe(true);
  });

  it("is enabled when ready with options and not pending", () => {
    expect(
      registerWorktreePathDisabled({ pending: false, loading: false, optionCount: 1 }),
    ).toBe(false);
  });
});
