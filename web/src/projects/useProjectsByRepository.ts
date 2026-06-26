import { useQuery } from "@tanstack/react-query";
import { listProjectsByRepository } from "@/api/gitGlobal";
import { gitQueryKeys } from "@/lib/gitQueryKeys";

export function useProjectsByRepository(
  repositoryId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.projectsByRepo(repositoryId),
    queryFn: ({ signal }) => listProjectsByRepository(repositoryId, { signal }),
    enabled: options?.enabled !== false && repositoryId.trim() !== "",
  });
}
