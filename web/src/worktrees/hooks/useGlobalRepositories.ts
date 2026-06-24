import { useQuery } from "@tanstack/react-query";
import { listGlobalGitRepositories } from "@/api/gitGlobal";
import { gitQueryKeys } from "../queryKeys";

export function useGlobalRepositories(options?: { enabled?: boolean }) {
  return useQuery({
    queryKey: gitQueryKeys.globalRepositories(),
    queryFn: ({ signal }) => listGlobalGitRepositories({ signal }),
    enabled: options?.enabled !== false,
  });
}
