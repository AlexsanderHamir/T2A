import { describe, expect, it } from "vitest";
import {
  buildGitContextItems,
  normalizeGitPath,
  shortSha,
} from "./commitDisplay";

describe("commitDisplay", () => {
  it("normalizes Windows and POSIX paths for comparison", () => {
    expect(normalizeGitPath("C:\\Users\\gomes\\T2A\\")).toBe(
      normalizeGitPath("C:/Users/gomes/T2A"),
    );
  });

  it("buildGitContextItems collapses duplicate repo and worktree", () => {
    const items = buildGitContextItems({
      repo: "C:\\Users\\gomes\\OneDrive\\Documents\\T2A",
      worktree: "C:/Users/gomes/OneDrive/Documents/T2A",
      branch: "main",
    });
    expect(items).toEqual([
      { label: "Branch", value: "main" },
      {
        label: "Worktree",
        value: "T2A",
        title: "C:/Users/gomes/OneDrive/Documents/T2A",
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
});
