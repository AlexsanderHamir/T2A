import { describe, expect, it } from "vitest";
import {
  formatReconcileSkippedReason,
  reconcileNeedsBindSummary,
  reconcileReportHasFollowUp,
  reconcileSkippedSummary,
} from "./reconcileReportDisplay";

describe("reconcileReportDisplay", () => {
  it("detects follow-up items", () => {
    expect(
      reconcileReportHasFollowUp({
        repo_path_updated: false,
        worktrees_path_updated: 0,
        worktrees_added: 0,
        worktrees_removed: 0,
        branches_head_updated: 0,
        worktrees_skipped: [],
        needs_branch_bind: [{ path: "/repo/wt", branch: "feature" }],
      }),
    ).toBe(true);
  });

  it("formats skipped reasons for operators", () => {
    expect(formatReconcileSkippedReason("branch_checkout_mismatch")).toMatch(/binding/i);
  });

  it("summarizes bind paths", () => {
    const lines = reconcileNeedsBindSummary({
      repo_path_updated: false,
      worktrees_path_updated: 0,
      worktrees_added: 0,
      worktrees_removed: 0,
      branches_head_updated: 0,
      worktrees_skipped: [],
      needs_branch_bind: [{ path: "/repo/feature", branch: "feature" }],
    });
    expect(lines[0]).toContain("/repo/feature");
  });

  it("summarizes skipped worktrees", () => {
    const lines = reconcileSkippedSummary({
      repo_path_updated: false,
      worktrees_path_updated: 0,
      worktrees_added: 0,
      worktrees_removed: 0,
      branches_head_updated: 0,
      worktrees_skipped: [
        { worktree_id: "00000000-0000-4000-8000-000000000099", reason: "has_task_ref" },
      ],
      needs_branch_bind: [],
    });
    expect(lines[0]).toMatch(/00000000/);
  });
});
