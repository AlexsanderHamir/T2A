import { useQuery } from "@tanstack/react-query";
import { listGitWorktrees } from "@/api/git";
import { gitQueryKeys } from "../queryKeys";

export function useWorktrees(
  projectId: string,
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.worktrees(projectId, repositoryId),
    queryFn: ({ signal }) => listGitWorktrees(projectId, repositoryId, { signal }),
    enabled:
      options?.enabled !== false &&
      projectId.trim() !== "" &&
      repositoryId.trim() !== "",
  });
}
