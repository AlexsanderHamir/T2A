import type { QueryClient } from "@tanstack/react-query";
import { listGlobalGitRepositories } from "@/api/gitGlobal";
import { gitQueryKeys } from "./queryKeys";

/** Returns true when at least one global git repository exists. Populates query cache. */
export async function ensureRepositoriesRegistered(
  queryClient: QueryClient,
): Promise<boolean> {
  const repositories = await queryClient.fetchQuery({
    queryKey: gitQueryKeys.globalRepositories(),
    queryFn: ({ signal }) => listGlobalGitRepositories({ signal }),
  });
  return repositories.length > 0;
}
