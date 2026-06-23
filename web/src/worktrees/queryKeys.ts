export const gitQueryKeys = {
  all: ["git"] as const,
  repositories: (projectId: string) =>
    [...gitQueryKeys.all, "repositories", projectId] as const,
  worktrees: (projectId: string, repositoryId: string) =>
    [...gitQueryKeys.all, "worktrees", projectId, repositoryId] as const,
  branches: (projectId: string, repositoryId: string) =>
    [...gitQueryKeys.all, "branches", projectId, repositoryId] as const,
};
