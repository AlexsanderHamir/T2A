import type { GitBranch, GitRepository, GitWorktree } from "@/types/git";
import type { JsonBodyType } from "msw";

export const FACTORY_GIT_REPO_ID = "00000000-0000-4000-8000-000000000010";
export const FACTORY_GIT_WORKTREE_ID = "00000000-0000-4000-8000-000000000020";
export const FACTORY_GIT_BRANCH_ID = "00000000-0000-4000-8000-000000000030";

export function gitRepositoryFactory(overrides: Partial<GitRepository> = {}): GitRepository {
  return {
    id: FACTORY_GIT_REPO_ID,
    path: "/repo/main",
    host_path: "",
    default_branch: "main",
    created_at: "2026-06-22T12:00:00Z",
    updated_at: "2026-06-22T12:00:00Z",
    ...overrides,
  };
}

export function gitWorktreeFactory(overrides: Partial<GitWorktree> = {}): GitWorktree {
  return {
    id: FACTORY_GIT_WORKTREE_ID,
    repository_id: FACTORY_GIT_REPO_ID,
    path: "/repo/main",
    name: "main",
    is_main: true,
    branch_id: FACTORY_GIT_BRANCH_ID,
    created_at: "2026-06-22T12:00:00Z",
    ...overrides,
  };
}

export function gitBranchFactory(overrides: Partial<GitBranch> = {}): GitBranch {
  return {
    id: FACTORY_GIT_BRANCH_ID,
    repository_id: FACTORY_GIT_REPO_ID,
    name: "main",
    head_sha: "abc123",
    created_at: "2026-06-22T12:00:00Z",
    ...overrides,
  };
}

export function globalGitRepositoriesResponse(): JsonBodyType {
  return { repositories: [gitRepositoryFactory()] };
}

export function globalGitWorktreesResponse(): JsonBodyType {
  return { worktrees: [gitWorktreeFactory()] };
}

export function globalGitBranchesResponse(): JsonBodyType {
  return { branches: [gitBranchFactory()] };
}

export function globalGitLiveBranchesResponse(): JsonBodyType {
  return { branches: [{ name: "main", head_sha: "abc123" }] };
}
