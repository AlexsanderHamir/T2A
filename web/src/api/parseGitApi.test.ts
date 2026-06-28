import { describe, expect, it } from "vitest";
import {
  parseGitBranch,
  parseGitBranchList,
  parseGitLiveBranchList,
  parseGitRepository,
  parseGitRepositoryList,
  parseGitReconcileResult,
  parseGitWorktree,
  parseGitWorktreeList,
} from "./parseGitApi";

const sampleRepo = {
  id: "00000000-0000-4000-8000-000000000010",
  path: "/repo/main",
  host_path: "",
  default_branch: "main",
  created_at: "2026-06-22T12:00:00Z",
  updated_at: "2026-06-22T12:00:00Z",
};

describe("parseGitApi", () => {
  it("parses repository list", () => {
    const rows = parseGitRepositoryList({ repositories: [sampleRepo] });
    expect(rows).toHaveLength(1);
    expect(rows[0]?.path).toBe("/repo/main");
  });

  it("parses single repository", () => {
    expect(parseGitRepository(sampleRepo).default_branch).toBe("main");
  });

  it("throws on malformed repository list", () => {
    expect(() => parseGitRepositoryList({ repositories: [{}] })).toThrow(/id/);
  });

  it("parses worktree list", () => {
    const rows = parseGitWorktreeList({
      worktrees: [
        {
          id: "00000000-0000-4000-8000-000000000020",
          repository_id: sampleRepo.id,
          path: "/repo/main",
          name: "main",
          is_main: true,
          created_at: "2026-06-22T12:00:00Z",
        },
      ],
    });
    expect(rows[0]?.is_main).toBe(true);
  });

  it("parses single worktree", () => {
    expect(
      parseGitWorktree({
        id: "00000000-0000-4000-8000-000000000020",
        repository_id: sampleRepo.id,
        path: "/repo/wt",
        name: "feature",
        is_main: false,
        created_at: "2026-06-22T12:00:00Z",
      }).name,
    ).toBe("feature");
  });

  it("parses branch list", () => {
    const rows = parseGitBranchList({
      branches: [
        {
          id: "00000000-0000-4000-8000-000000000030",
          repository_id: sampleRepo.id,
          name: "main",
          head_sha: "abc123",
          created_at: "2026-06-22T12:00:00Z",
        },
      ],
    });
    expect(rows[0]?.head_sha).toBe("abc123");
  });

  it("throws on malformed branch", () => {
    expect(() => parseGitBranch({})).toThrow(/id/);
  });

  it("parses worktree with branch_id", () => {
    const wt = parseGitWorktree({
      id: "00000000-0000-4000-8000-000000000020",
      repository_id: sampleRepo.id,
      path: "/repo/wt",
      name: "feature",
      is_main: false,
      branch_id: "00000000-0000-4000-8000-000000000030",
      created_at: "2026-06-22T12:00:00Z",
    });
    expect(wt.branch_id).toBe("00000000-0000-4000-8000-000000000030");
  });

  it("parses worktree list when branch_id is empty string from Go JSON", () => {
    const rows = parseGitWorktreeList({
      worktrees: [
        {
          id: "00000000-0000-4000-8000-000000000020",
          repository_id: sampleRepo.id,
          path: "/repo/main",
          name: "main",
          is_main: true,
          branch_id: "",
          created_at: "2026-06-22T12:00:00Z",
        },
        {
          id: "00000000-0000-4000-8000-000000000021",
          repository_id: sampleRepo.id,
          path: "/repo/wt-01",
          name: "wt-01",
          is_main: false,
          branch_id: "00000000-0000-4000-8000-000000000030",
          created_at: "2026-06-22T12:00:00Z",
        },
      ],
    });
    expect(rows).toHaveLength(2);
    expect(rows[0]?.branch_id).toBeUndefined();
    expect(rows[1]?.branch_id).toBe("00000000-0000-4000-8000-000000000030");
  });

  it("parses live branch list", () => {
    const rows = parseGitLiveBranchList({
      branches: [{ name: "main", head_sha: "deadbeef" }],
    });
    expect(rows[0]?.name).toBe("main");
  });

  it("parses reconcile result with report", () => {
    const result = parseGitReconcileResult({
      status: "ok",
      report: {
        repo_path_updated: true,
        worktrees_path_updated: 2,
        worktrees_added: 1,
        worktrees_removed: 0,
        branches_head_updated: 3,
        worktrees_skipped: [{ worktree_id: "00000000-0000-4000-8000-000000000020", reason: "has_tasks" }],
        needs_branch_bind: [{ path: "/repo/feature", branch: "feature" }],
      },
    });
    expect(result.status).toBe("ok");
    expect(result.report.repo_path_updated).toBe(true);
    expect(result.report.worktrees_path_updated).toBe(2);
    expect(result.report.worktrees_skipped).toHaveLength(1);
    expect(result.report.needs_branch_bind[0]?.branch).toBe("feature");
  });
});
