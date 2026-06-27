import { describe, expect, it } from "vitest";
import {
  parseGitBranch,
  parseGitBranchList,
  parseGitLiveBranchList,
  parseGitRepository,
  parseGitRepositoryList,
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

  it("parses live branch list", () => {
    const rows = parseGitLiveBranchList({
      branches: [{ name: "main", head_sha: "deadbeef" }],
    });
    expect(rows[0]?.name).toBe("main");
  });
});
