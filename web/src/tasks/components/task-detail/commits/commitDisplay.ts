import type { CommitStatus } from "@/types";

export type GitContextFields = {
  repo: string;
  worktree: string;
  branch: string;
};

export type GitContextItem = {
  label: string;
  value: string;
  title?: string;
};

/** Normalize paths so Windows separators compare equal to git output. */
export function normalizeGitPath(path: string): string {
  return path.trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
}

export function pathTail(path: string): string {
  const normalized = path.replace(/\\/g, "/").replace(/\/+$/, "");
  const parts = normalized.split("/").filter(Boolean);
  return parts.length > 0 ? parts[parts.length - 1] : path;
}

export function shortSha(sha: string): string {
  const trimmed = sha.trim();
  return trimmed.length > 7 ? trimmed.slice(0, 7) : trimmed;
}

/** Labeled repo context for the commits panel (avoids ambiguous breadcrumbs). */
export function buildGitContextItems(ctx: GitContextFields): GitContextItem[] {
  const branch = ctx.branch.trim() || "detached";
  const repo = ctx.repo.trim();
  const worktree = ctx.worktree.trim();
  const repoNorm = normalizeGitPath(repo);
  const worktreeNorm = normalizeGitPath(worktree);
  const items: GitContextItem[] = [{ label: "Branch", value: branch }];

  if (worktreeNorm !== "" && worktreeNorm !== repoNorm) {
    items.push({
      label: "Worktree",
      value: pathTail(worktree),
      title: worktree,
    });
    if (repoNorm !== "") {
      items.push({
        label: "Repo root",
        value: pathTail(repo),
        title: repo,
      });
    }
  } else {
    const primary = worktree || repo;
    if (primary !== "") {
      items.push({
        label: "Worktree",
        value: pathTail(primary),
        title: primary,
      });
    }
  }

  return items;
}

export function commitStatusLabel(status: CommitStatus): string {
  switch (status) {
    case "eligible":
      return "Eligible";
    case "observed":
      return "Observed";
    case "inherited":
      return "Inherited";
    case "superseded":
      return "Superseded";
    default:
      return status;
  }
}

export function commitStatusPillClass(status: CommitStatus): string {
  switch (status) {
    case "eligible":
      return "cell-pill cell-pill--commit-eligible";
    case "observed":
      return "cell-pill cell-pill--commit-observed";
    case "inherited":
      return "cell-pill cell-pill--commit-inherited";
    case "superseded":
      return "cell-pill cell-pill--commit-superseded";
    default:
      return "cell-pill";
  }
}
