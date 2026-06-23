import { useQuery } from "@tanstack/react-query";
import { listGitBranches } from "@/api/git";
import { gitQueryKeys } from "../queryKeys";

export function useBranches(
  projectId: string,
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.branches(projectId, repositoryId),
    queryFn: ({ signal }) => listGitBranches(projectId, repositoryId, { signal }),
    enabled:
      options?.enabled !== false &&
      projectId.trim() !== "" &&
      repositoryId.trim() !== "",
  });
}
