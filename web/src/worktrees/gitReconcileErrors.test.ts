import { describe, expect, it } from "vitest";
import { ApiError } from "@/api";
import { formatReconcileSuccess, gitReconcileErrorMessage } from "./gitReconcileErrors";

describe("gitReconcileErrors", () => {
  it("maps bootstrap mismatch to operator guidance", () => {
    const err = new ApiError("wrong repo", { status: 409, code: "bootstrap_mismatch" });
    expect(gitReconcileErrorMessage(err)).toMatch(/same repository/i);
  });

  it("summarizes reconcile report counts", () => {
    const message = formatReconcileSuccess({
      status: "ok",
      report: {
        repo_path_updated: true,
        worktrees_path_updated: 1,
        worktrees_added: 0,
        worktrees_removed: 0,
        branches_head_updated: 2,
        worktrees_skipped: [],
        needs_branch_bind: [],
      },
    });
    expect(message).toMatch(/repository path updated/);
    expect(message).toMatch(/1 worktree path updated/);
    expect(message).toMatch(/2 branch heads refreshed/);
  });
});
