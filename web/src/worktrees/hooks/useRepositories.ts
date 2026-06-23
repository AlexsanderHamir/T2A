import { useQuery } from "@tanstack/react-query";
import { listGitRepositories } from "@/api/git";
import { gitQueryKeys } from "../queryKeys";

export function useRepositories(projectId: string, options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: gitQueryKeys.repositories(projectId),
    queryFn: ({ signal }) => listGitRepositories(projectId, { signal }),
    enabled: options?.enabled !== false && projectId.trim() !== "",
  });
}
