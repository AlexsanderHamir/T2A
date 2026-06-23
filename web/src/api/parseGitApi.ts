import type { GitBranch, GitRepository, GitReconcileResult, GitWorktree } from "@/types/git";
import { isRecord, parseNonEmptyString, parseString } from "./parseTaskApiCore";

function parseGitRepositoryRow(value: unknown, path: string): GitRepository {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    id: parseNonEmptyString(value.id, `${path}.id`),
    project_id: parseNonEmptyString(value.project_id, `${path}.project_id`),
    path: parseString(value.path, `${path}.path`),
    host_path: parseString(value.host_path, `${path}.host_path`),
    default_branch: parseString(value.default_branch, `${path}.default_branch`),
    created_at: parseString(value.created_at, `${path}.created_at`),
    updated_at: parseString(value.updated_at, `${path}.updated_at`),
  };
}

function parseGitWorktreeRow(value: unknown, path: string): GitWorktree {
  if (!isRecord(value)) {
    throw new Error(`Invalid API response: ${path} must be object`);
  }
  return {
    id: parseNonEmptyString(value.id, `${path}.id`),
    repository_id: parseNonEmptyString(value.repository_id, `${path}.repository_id`),
    path: parseString(value.path, `${path}.path`),
    name: parseString(value.name, `${path}.name`),
    is_main: Boolean(value.is_main),
    created_at: parseString(value.created_at, `${path}.created_at`),
  };
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

export function parseGitReconcileResult(raw: unknown): GitReconcileResult {
  if (!isRecord(raw)) {
    throw new Error("Invalid API response: body must be object");
  }
  return {
    status: parseString(raw.status, "status"),
  };
}
