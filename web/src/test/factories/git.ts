import type { GitBranch, GitRepository, GitWorktree, WorktreeBranch } from "@/types/git";

export const FACTORY_GIT_REPO_ID = "00000000-0000-4000-8000-000000000010";
export const FACTORY_GIT_WORKTREE_ID = "00000000-0000-4000-8000-000000000020";
export const FACTORY_GIT_BRANCH_ID = "00000000-0000-4000-8000-000000000030";
export const FACTORY_WORKTREE_BRANCH_ID = "00000000-0000-4000-8000-000000000040";

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

export function worktreeBranchFactory(overrides: Partial<WorktreeBranch> = {}): WorktreeBranch {
  return {
    id: FACTORY_WORKTREE_BRANCH_ID,
    worktree_id: FACTORY_GIT_WORKTREE_ID,
    branch_id: FACTORY_GIT_BRANCH_ID,
    created_at: "2026-06-22T12:00:00Z",
    ...overrides,
  };
}

export function globalGitRepositoriesResponse(): unknown {
  return { repositories: [gitRepositoryFactory()] };
}

export function globalGitWorktreesResponse(): unknown {
  return { worktrees: [gitWorktreeFactory()] };
}

export function globalGitBranchesResponse(): unknown {
  return { branches: [gitBranchFactory()] };
}

export function globalGitLiveBranchesResponse(): unknown {
  return { branches: [{ name: "main", head_sha: "abc123" }] };
}

export function worktreeBranchAssociationsResponse(): unknown {
  return { associations: [worktreeBranchFactory()] };
}
