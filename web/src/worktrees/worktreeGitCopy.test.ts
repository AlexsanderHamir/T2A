import { describe, expect, it } from "vitest";
import {
  deleteWorktreeAriaLabel,
  liveWorktreeOptionLabel,
  worktreeAriaLabel,
  worktreeGitCopy,
} from "./worktreeGitCopy";

describe("worktreeGitCopy", () => {
  it("uses git-standard row labels", () => {
    expect(worktreeGitCopy.mainWorktreeLabel).toBe("main worktree");
    expect(worktreeGitCopy.detachedHead).toBe("Detached HEAD");
  });

  it("formats aria labels for worktree rows", () => {
    expect(worktreeAriaLabel("feature")).toBe("Worktree: feature");
    expect(deleteWorktreeAriaLabel("feature")).toBe('Delete worktree "feature"');
  });

  it("labels live worktree options with main worktree hint", () => {
    expect(liveWorktreeOptionLabel("/repo/main", true)).toBe(
      "/repo/main (main worktree)",
    );
    expect(liveWorktreeOptionLabel("/repo/feature", false)).toBe("/repo/feature");
  });
});
