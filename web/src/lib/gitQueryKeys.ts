export const gitQueryKeys = {
  all: ["git"] as const,
  /** Legacy project-scoped keys (deprecated; use global keys in new UI). */
  repositories: (projectId: string) =>
    [...gitQueryKeys.all, "repositories", projectId] as const,
  worktrees: (projectId: string, repositoryId: string) =>
    [...gitQueryKeys.all, "worktrees", projectId, repositoryId] as const,
  branches: (projectId: string, repositoryId: string) =>
    [...gitQueryKeys.all, "branches", projectId, repositoryId] as const,
  /** Global git tree (ADR-0037). */
  globalRepositories: () => [...gitQueryKeys.all, "global", "repositories"] as const,
  globalWorktrees: (repositoryId: string) =>
    [...gitQueryKeys.all, "global", "worktrees", repositoryId] as const,
  globalBranches: (repositoryId: string) =>
    [...gitQueryKeys.all, "global", "branches", repositoryId] as const,
  globalLiveBranches: (repositoryId: string) =>
    [...gitQueryKeys.all, "global", "branches", "live", repositoryId] as const,
  worktreeAssociations: (worktreeId: string) =>
    [...gitQueryKeys.all, "global", "associations", worktreeId] as const,
  projectsByRepo: (repositoryId: string) =>
    [...gitQueryKeys.all, "global", "projects", repositoryId] as const,
};
