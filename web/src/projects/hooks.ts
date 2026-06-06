import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import {
  getProject,
  listProjectContext,
  listProjects,
} from "@/api";
import type {
  Project,
  ProjectContextListResponse,
  ProjectListResponse,
} from "@/types";
import { projectQueryKeys } from "./queryKeys";

export function useProjects(options?: {
  includeArchived?: boolean;
  limit?: number;
  /**
   * When false, the query stays mounted (so the cache survives) but
   * does not trigger a network fetch — useful for shell-level callers
   * that want the data lazily, only after a user opens a modal or a
   * project picker.
   */
  enabled?: boolean;
}): UseQueryResult<ProjectListResponse, Error> {
  const includeArchived = options?.includeArchived ?? false;
  const limit = options?.limit ?? 50;
  const enabled = options?.enabled ?? true;
  return useQuery({
    queryKey: projectQueryKeys.list(includeArchived, limit),
    queryFn: ({ signal }) =>
      listProjects({
        signal,
        includeArchived,
        limit,
      }),
    enabled,
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
