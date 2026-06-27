import { describe, expect, it } from "vitest";
import type { GitLiveWorktree, GitWorktree } from "@/types/git";
import { worktreeStatusLabel } from "./worktreeStatus";

const baseWorktree: GitWorktree = {
  id: "00000000-0000-4000-8000-000000000020",
  repository_id: "00000000-0000-4000-8000-000000000010",
  path: "/repo/feature",
  name: "feature",
  is_main: false,
  branch_id: "00000000-0000-4000-8000-000000000030",
  created_at: "2026-06-22T12:00:00Z",
};

describe("worktreeStatusLabel", () => {
  it("reports needs branch bind when branch_id is missing", () => {
    const unbound = { ...baseWorktree, branch_id: "" };
    const label = worktreeStatusLabel(undefined, unbound);
    expect(label.label).toMatch(/needs branch bind/i);
  });

  it("reports detached HEAD from live git metadata", () => {
    const live: GitLiveWorktree = {
      path: "/repo/feature",
      branch: "",
      is_main: false,
      detached: true,
      registered: true,
      locked: false,
      prunable: false,
    };
    const label = worktreeStatusLabel(live, baseWorktree);
    expect(label.label).toMatch(/detached head/i);
  });

  it("reports locked before ready", () => {
    const live: GitLiveWorktree = {
      path: "/repo/feature",
      branch: "feature",
      is_main: false,
      detached: false,
      registered: true,
      locked: true,
      prunable: false,
    };
    expect(worktreeStatusLabel(live, baseWorktree).label).toMatch(/locked/i);
  });
});
