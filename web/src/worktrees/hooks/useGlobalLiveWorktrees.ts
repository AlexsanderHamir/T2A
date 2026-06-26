import { useQuery } from "@tanstack/react-query";
import { listGlobalGitLiveWorktrees } from "@/api/gitGlobal";
import { gitQueryKeys } from "../queryKeys";

export function useGlobalLiveWorktrees(
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.globalLiveWorktrees(repositoryId),
    queryFn: ({ signal }) => listGlobalGitLiveWorktrees(repositoryId, { signal }),
    enabled: options?.enabled !== false && repositoryId.trim() !== "",
  });
}
