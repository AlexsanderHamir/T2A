import { useQuery } from "@tanstack/react-query";
import { listGlobalGitBranches, listGlobalGitLiveBranches } from "@/api/gitGlobal";
import { gitQueryKeys } from "../queryKeys";

export function useGlobalBranches(
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.globalBranches(repositoryId),
    queryFn: ({ signal }) => listGlobalGitBranches(repositoryId, { signal }),
    enabled: options?.enabled !== false && repositoryId.trim() !== "",
  });
}

export function useGlobalLiveBranches(
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.globalLiveBranches(repositoryId),
    queryFn: ({ signal }) => listGlobalGitLiveBranches(repositoryId, { signal }),
    enabled: options?.enabled !== false && repositoryId.trim() !== "",
  });
}
