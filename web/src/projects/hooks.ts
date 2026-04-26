import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import { getProject, listProjectContext, listProjects } from "@/api";
import type {
  Project,
  ProjectContextListResponse,
  ProjectListResponse,
} from "@/types";
import { projectQueryKeys } from "./queryKeys";

export function useProjects(options?: {
  includeArchived?: boolean;
  limit?: number;
}): UseQueryResult<ProjectListResponse, Error> {
  const includeArchived = options?.includeArchived ?? false;
  const limit = options?.limit ?? 50;
  return useQuery({
    queryKey: projectQueryKeys.list(includeArchived, limit),
    queryFn: ({ signal }) =>
      listProjects({
        signal,
        includeArchived,
        limit,
      }),
  });
}

export function useProject(
  projectId: string,
  options?: { enabled?: boolean },
): UseQueryResult<Project, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(projectId);
  return useQuery({
    queryKey: projectQueryKeys.detail(projectId),
    queryFn: ({ signal }) => getProject(projectId, { signal }),
    enabled,
  });
}

export function useProjectContext(
  projectId: string,
  options?: { enabled?: boolean; limit?: number; pinnedOnly?: boolean },
): UseQueryResult<ProjectContextListResponse, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(projectId);
  return useQuery({
    queryKey: projectQueryKeys.context(projectId),
    queryFn: ({ signal }) =>
      listProjectContext(projectId, {
        signal,
        limit: options?.limit,
        pinnedOnly: options?.pinnedOnly,
      }),
    enabled,
  });
}
