import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  createGitBranch,
  createGitRepository,
  createGitWorktree,
  deleteGitBranch,
  deleteGitRepository,
  deleteGitWorktree,
  reconcileGitRepository,
} from "@/api/git";
import { gitQueryKeys } from "../queryKeys";

export function useGitMutations(projectId: string) {
  const queryClient = useQueryClient();

  const invalidateRepo = (repositoryId: string) => {
    void queryClient.invalidateQueries({
      queryKey: gitQueryKeys.repositories(projectId),
    });
    void queryClient.invalidateQueries({
      queryKey: gitQueryKeys.worktrees(projectId, repositoryId),
    });
    void queryClient.invalidateQueries({
      queryKey: gitQueryKeys.branches(projectId, repositoryId),
    });
  };

  const registerRepository = useMutation({
    mutationFn: (input: { path: string; host_path?: string; default_branch?: string }) =>
      createGitRepository(projectId, input),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: gitQueryKeys.repositories(projectId),
      });
    },
  });

  const removeRepository = useMutation({
    mutationFn: (repositoryId: string) => deleteGitRepository(projectId, repositoryId),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: gitQueryKeys.repositories(projectId),
      });
    },
  });

  const addWorktree = useMutation({
    mutationFn: (input: {
      repositoryId: string;
      path: string;
      name?: string;
      branch: string;
      create_branch?: boolean;
      start_point?: string;
    }) =>
      createGitWorktree(projectId, input.repositoryId, {
        path: input.path,
        name: input.name,
        branch: input.branch,
        create_branch: input.create_branch,
        start_point: input.start_point,
      }),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const removeWorktree = useMutation({
    mutationFn: (input: { worktreeId: string; repositoryId: string; force?: boolean }) =>
      deleteGitWorktree(projectId, input.worktreeId, { force: input.force }),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const addBranch = useMutation({
    mutationFn: (input: { repositoryId: string; name: string; start_point?: string }) =>
      createGitBranch(projectId, input.repositoryId, {
        name: input.name,
        start_point: input.start_point,
      }),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const removeBranch = useMutation({
    mutationFn: (input: { branchId: string; repositoryId: string; force?: boolean }) =>
      deleteGitBranch(projectId, input.branchId, { force: input.force }),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const reconcile = useMutation({
    mutationFn: (repositoryId: string) => reconcileGitRepository(projectId, repositoryId),
    onSuccess: (_data, repositoryId) => invalidateRepo(repositoryId),
  });

  return {
    registerRepository,
    removeRepository,
    addWorktree,
    removeWorktree,
    addBranch,
    removeBranch,
    reconcile,
  };
}
