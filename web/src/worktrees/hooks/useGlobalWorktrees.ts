import { useQuery } from "@tanstack/react-query";
import { listGlobalGitWorktrees } from "@/api/gitGlobal";
import { gitQueryKeys } from "../queryKeys";

export function useGlobalWorktrees(
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.globalWorktrees(repositoryId),
    queryFn: ({ signal }) => listGlobalGitWorktrees(repositoryId, { signal }),
    enabled: options?.enabled !== false && repositoryId.trim() !== "",
  });
}
