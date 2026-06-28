import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  createGlobalGitRepository,
  createGlobalGitWorktree,
  deleteGlobalGitRepository,
  unregisterGlobalGitWorktree,
  reconcileGlobalGitRepository,
  registerGlobalGitWorktree,
  relocateGlobalGitRepository,
} from "@/api/gitGlobal";
import type { GitReconcileInput } from "@/types/git";
import { gitQueryKeys } from "../queryKeys";

export function useGlobalGitMutations() {
  const qc = useQueryClient();

  const invalidateRepo = (repositoryId: string) => {
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalRepositories() });
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalRepository(repositoryId) });
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalWorktrees(repositoryId) });
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalBranches(repositoryId) });
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalLiveBranches(repositoryId) });
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalLiveWorktrees(repositoryId) });
    void qc.invalidateQueries({ queryKey: gitQueryKeys.projectsByRepo(repositoryId) });
  };

  const createRepository = useMutation({
    mutationFn: createGlobalGitRepository,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: gitQueryKeys.globalRepositories() });
    },
  });

  const deleteRepository = useMutation({
    mutationFn: deleteGlobalGitRepository,
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: gitQueryKeys.globalRepositories() });
    },
  });

  const createWorktree = useMutation({
    mutationFn: (vars: {
      repositoryId: string;
      input: Parameters<typeof createGlobalGitWorktree>[1];
    }) => createGlobalGitWorktree(vars.repositoryId, vars.input),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const registerWorktree = useMutation({
    mutationFn: (vars: {
      repositoryId: string;
      input: Parameters<typeof registerGlobalGitWorktree>[1];
    }) => registerGlobalGitWorktree(vars.repositoryId, vars.input),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const unregisterWorktree = useMutation({
    mutationFn: (vars: { worktreeId: string; repositoryId: string }) =>
      unregisterGlobalGitWorktree(vars.worktreeId),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const reconcile = useMutation({
    mutationFn: (vars: { repositoryId: string; input?: GitReconcileInput }) =>
      reconcileGlobalGitRepository(vars.repositoryId, vars.input),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const relocateRepository = useMutation({
    mutationFn: (vars: { repositoryId: string; input: { path: string } }) =>
      relocateGlobalGitRepository(vars.repositoryId, vars.input),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  return {
    createRepository,
    deleteRepository,
    createWorktree,
    registerWorktree,
    unregisterWorktree,
    reconcile,
    relocateRepository,
  };
}
