import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { fetchRepoCommitDiff, repoQueryKeys, type RepoDiffResult } from "@/api/repo";

export function useCommitDiff(
  sha: string,
  options?: { enabled?: boolean },
): UseQueryResult<RepoDiffResult | null, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(sha);
  return useQuery({
    queryKey: repoQueryKeys.diff(sha),
    queryFn: ({ signal }) => fetchRepoCommitDiff(sha, { signal }),
    enabled,
    staleTime: Number.POSITIVE_INFINITY,
    gcTime: 30 * 60_000,
    retry: 1,
  });
}
