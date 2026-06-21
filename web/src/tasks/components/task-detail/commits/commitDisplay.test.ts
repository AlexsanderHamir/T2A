import { describe, expect, it } from "vitest";
import {
  buildGitContextItems,
  normalizeGitPath,
  shortSha,
  taskCommitDiffPath,
} from "./commitDisplay";

describe("commitDisplay", () => {
  it("normalizes Windows and POSIX paths for comparison", () => {
    expect(normalizeGitPath("C:\\tmp\\hamix-repo\\")).toBe(
      normalizeGitPath("C:/tmp/hamix-repo"),
    );
  });

  it("buildGitContextItems collapses duplicate repo and worktree", () => {
    const items = buildGitContextItems({
      repo: "C:\\tmp\\hamix-repo",
      worktree: "C:/tmp/hamix-repo",
      branch: "main",
    });
    expect(items).toEqual([
      { label: "Branch", value: "main" },
      {
        label: "Worktree",
        value: "hamix-repo",
        title: "C:/tmp/hamix-repo",
      },
    ]);
  });

  it("buildGitContextItems shows separate worktree and repo when they differ", () => {
    const items = buildGitContextItems({
      repo: "/workspace/monorepo",
      worktree: "/workspace/monorepo/apps/web",
      branch: "feature/commits",
    });
    expect(items.map((i) => i.label)).toEqual(["Branch", "Worktree", "Repo root"]);
  });

  it("shortSha trims to seven characters", () => {
    expect(shortSha("0fc23bf2d0b5d5e8fc0d3638df57ac4de38053c1")).toBe("0fc23bf");
  });

  it("taskCommitDiffPath encodes task and sha segments", () => {
    expect(taskCommitDiffPath("task 1", "abc1234")).toBe(
      "/tasks/task%201/commits/abc1234",
    );
  });
});
