import { useQuery, type UseQueryResult } from "@tanstack/react-query";
import {
  getProject,
  listProjectContext,
  listProjectGoals,
  listProjectSteps,
  listProjects,
} from "@/api";
import type {
  Project,
  ProjectContextListResponse,
  ProjectGoalsListResponse,
  ProjectListResponse,
  ProjectStepsListResponse,
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

export function useProjectSteps(
  projectId: string,
  options?: { enabled?: boolean; goalId?: string },
): UseQueryResult<ProjectStepsListResponse, Error> {
  const gid = (options?.goalId ?? "").trim();
  const enabled = (options?.enabled ?? true) && Boolean(projectId);
  return useQuery({
    queryKey: projectQueryKeys.steps(projectId, gid),
    queryFn: ({ signal }) =>
      listProjectSteps(projectId, { signal, goalId: gid !== "" ? gid : undefined }),
    enabled,
  });
}

export function useProjectGoals(
  projectId: string,
  options?: { enabled?: boolean },
): UseQueryResult<ProjectGoalsListResponse, Error> {
  const enabled = (options?.enabled ?? true) && Boolean(projectId);
  return useQuery({
    queryKey: projectQueryKeys.goals(projectId),
    queryFn: ({ signal }) => listProjectGoals(projectId, { signal }),
    enabled,
  });
}
