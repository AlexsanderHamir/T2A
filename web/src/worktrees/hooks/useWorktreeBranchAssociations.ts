import { useQuery } from "@tanstack/react-query";
import { listWorktreeBranchAssociations } from "@/api/gitGlobal";
import { gitQueryKeys } from "../queryKeys";

export function useWorktreeBranchAssociations(
  worktreeId: string,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: gitQueryKeys.worktreeAssociations(worktreeId),
    queryFn: ({ signal }) => listWorktreeBranchAssociations(worktreeId, { signal }),
    enabled: options?.enabled !== false && worktreeId.trim() !== "",
  });
}
