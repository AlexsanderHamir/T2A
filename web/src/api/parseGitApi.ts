import type {
  GitBranch,
  GitLiveBranch,
  GitLiveWorktree,
  GitRepository,
  GitReconcileNeedsBranchBind,
  GitReconcileReport,
  GitReconcileResult,
  GitReconcileSkipped,
  GitWorktree,
  GitWorktreeProbe,
} from "@/types/git";
import { isRecord, parseNonEmptyString, parseOptionalNonEmptyId, parseString } from "./parseTaskApiCore";

function parseGitRepositoryRow(value: unknown, path: string): GitRepository {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    id: parseNonEmptyString(value.id, `${path}.id`),
    path: parseString(value.path, `${path}.path`),
    git_common_dir: isRecord(value) && value.git_common_dir != null
      ? parseString(value.git_common_dir, `${path}.git_common_dir`)
      : "",
    host_path: parseString(value.host_path, `${path}.host_path`),
    default_branch: isRecord(value) && value.default_branch != null
      ? parseString(value.default_branch, `${path}.default_branch`)
      : "",
    created_at: parseString(value.created_at, `${path}.created_at`),
    updated_at: parseString(value.updated_at, `${path}.updated_at`),
  };
}

function parseGitWorktreeRow(value: unknown, path: string): GitWorktree {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  const row: GitWorktree = {
    id: parseNonEmptyString(value.id, `${path}.id`),
    repository_id: parseNonEmptyString(value.repository_id, `${path}.repository_id`),
    path: parseString(value.path, `${path}.path`),
    name: parseString(value.name, `${path}.name`),
    is_main: Boolean(value.is_main),
    created_at: parseString(value.created_at, `${path}.created_at`),
  };
  const branchID = parseOptionalNonEmptyId(value.branch_id, `${path}.branch_id`);
  if (branchID) {
    row.branch_id = branchID;
  }
  return row;
}

function parseGitBranchRow(value: unknown, path: string): GitBranch {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    id: parseNonEmptyString(value.id, `${path}.id`),
    repository_id: parseNonEmptyString(value.repository_id, `${path}.repository_id`),
    name: parseString(value.name, `${path}.name`),
    head_sha: parseString(value.head_sha, `${path}.head_sha`),
    created_at: parseString(value.created_at, `${path}.created_at`),
  };
}

function parseGitLiveBranchRow(value: unknown, path: string): GitLiveBranch {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    name: parseString(value.name, `${path}.name`),
    head_sha: parseString(value.head_sha, `${path}.head_sha`),
  };
}

export function parseGitRepositoryList(raw: unknown): GitRepository[] {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  const rows = raw.repositories;
  if (!Array.isArray(rows)) {
    throw new Error("Invalid API response: repositories must be array");
  }
  return rows.map((row, i) => parseGitRepositoryRow(row, `repositories[${i}]`));
}

export function parseGitRepository(raw: unknown): GitRepository {
  return parseGitRepositoryRow(raw, "repository");
}

export function parseGitWorktreeList(raw: unknown): GitWorktree[] {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  const rows = raw.worktrees;
  if (!Array.isArray(rows)) {
    throw new Error("Invalid API response: worktrees must be array");
  }
  return rows.map((row, i) => parseGitWorktreeRow(row, `worktrees[${i}]`));
}

export function parseGitWorktree(raw: unknown): GitWorktree {
  return parseGitWorktreeRow(raw, "worktree");
}

export function parseGitBranchList(raw: unknown): GitBranch[] {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  const rows = raw.branches;
  if (!Array.isArray(rows)) {
    throw new Error("Invalid API response: branches must be array");
  }
  return rows.map((row, i) => parseGitBranchRow(row, `branches[${i}]`));
}

export function parseGitBranch(raw: unknown): GitBranch {
  return parseGitBranchRow(raw, "branch");
}

export function parseGitLiveBranchList(raw: unknown): GitLiveBranch[] {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  const rows = raw.branches;
  if (!Array.isArray(rows)) {
    throw new Error("Invalid API response: branches must be array");
  }
  return rows.map((row, i) => parseGitLiveBranchRow(row, `branches[${i}]`));
}

function parseGitLiveWorktreeRow(value: unknown, path: string): GitLiveWorktree {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    path: parseString(value.path, `${path}.path`),
    branch: parseString(value.branch, `${path}.branch`),
    is_main: Boolean(value.is_main),
    detached: Boolean(value.detached),
    registered: Boolean(value.registered),
  };
}

export function parseGitLiveWorktreeList(raw: unknown): GitLiveWorktree[] {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  const rows = raw.worktrees;
  if (!Array.isArray(rows)) {
    throw new Error("Invalid API response: worktrees must be array");
  }
  return rows.map((row, i) => parseGitLiveWorktreeRow(row, `worktrees[${i}]`));
}

export function parseGitWorktreeProbe(raw: unknown): GitWorktreeProbe {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  return {
    path: parseString(raw.path, "path"),
    linked: Boolean(raw.linked),
    is_main: Boolean(raw.is_main),
    branch: parseString(raw.branch, "branch"),
    registered: Boolean(raw.registered),
  };
}

function parseGitReconcileSkipped(value: unknown, path: string): GitReconcileSkipped {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    worktree_id: parseNonEmptyString(value.worktree_id, `${path}.worktree_id`),
    reason: parseString(value.reason, `${path}.reason`),
  };
}

function parseGitReconcileNeedsBranchBind(
  value: unknown,
  path: string,
): GitReconcileNeedsBranchBind {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    path: parseString(value.path, `${path}.path`),
    branch: parseString(value.branch, `${path}.branch`),
  };
}

function parseGitReconcileReport(value: unknown): GitReconcileReport {
  if (!isRecord(value)) {
    throw new Error("Invalid API response: report must be object");
  }
  const skippedRaw = value.worktrees_skipped;
  const needsBindRaw = value.needs_branch_bind;
  const skipped: GitReconcileSkipped[] = Array.isArray(skippedRaw)
    ? skippedRaw.map((row, i) => parseGitReconcileSkipped(row, `report.worktrees_skipped[${i}]`))
    : [];
  const needsBranchBind: GitReconcileNeedsBranchBind[] = Array.isArray(needsBindRaw)
    ? needsBindRaw.map((row, i) =>
        parseGitReconcileNeedsBranchBind(row, `report.needs_branch_bind[${i}]`),
      )
    : [];
  return {
    repo_path_updated: value.repo_path_updated === true,
    worktrees_path_updated:
      typeof value.worktrees_path_updated === "number" && Number.isFinite(value.worktrees_path_updated)
        ? value.worktrees_path_updated
        : 0,
    worktrees_added:
      typeof value.worktrees_added === "number" && Number.isFinite(value.worktrees_added)
        ? value.worktrees_added
        : 0,
    worktrees_removed:
      typeof value.worktrees_removed === "number" && Number.isFinite(value.worktrees_removed)
        ? value.worktrees_removed
        : 0,
    branches_head_updated:
      typeof value.branches_head_updated === "number" && Number.isFinite(value.branches_head_updated)
        ? value.branches_head_updated
        : 0,
    worktrees_skipped: skipped,
    needs_branch_bind: needsBranchBind,
  };
}

export function parseGitReconcileResult(raw: unknown): GitReconcileResult {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  return {
    status: parseString(raw.status, "status"),
    report: parseGitReconcileReport(raw.report),
  };
}
