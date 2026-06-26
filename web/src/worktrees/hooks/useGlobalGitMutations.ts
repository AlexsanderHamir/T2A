import { useMutation, useQueryClient } from "@tanstack/react-query";
import {
  associateWorktreeBranch,
  createGlobalGitRepository,
  createGlobalGitWorktree,
  deleteGlobalGitRepository,
  deleteGlobalGitWorktree,
  reconcileGlobalGitRepository,
  registerGlobalGitWorktree,
  removeWorktreeBranchAssociation,
} from "@/api/gitGlobal";
import { gitQueryKeys } from "../queryKeys";

export function useGlobalGitMutations() {
  const qc = useQueryClient();

  const invalidateRepo = (repositoryId: string) => {
    void qc.invalidateQueries({ queryKey: gitQueryKeys.globalRepositories() });
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

  const deleteWorktree = useMutation({
    mutationFn: (vars: { worktreeId: string; repositoryId: string; force?: boolean }) =>
      deleteGlobalGitWorktree(vars.worktreeId, { force: vars.force }),
    onSuccess: (_data, vars) => invalidateRepo(vars.repositoryId),
  });

  const associateBranch = useMutation({
    mutationFn: (vars: {
      worktreeId: string;
      repositoryId: string;
      input: Parameters<typeof associateWorktreeBranch>[1];
    }) => associateWorktreeBranch(vars.worktreeId, vars.input),
    onSuccess: (_data, vars) => {
      invalidateRepo(vars.repositoryId);
      void qc.invalidateQueries({
        queryKey: gitQueryKeys.worktreeAssociations(vars.worktreeId),
      });
    },
  });

  const removeAssociation = useMutation({
    mutationFn: (vars: { worktreeId: string; branchId: string; repositoryId: string }) =>
      removeWorktreeBranchAssociation(vars.worktreeId, vars.branchId),
    onSuccess: (_data, vars) => {
      invalidateRepo(vars.repositoryId);
      void qc.invalidateQueries({
        queryKey: gitQueryKeys.worktreeAssociations(vars.worktreeId),
      });
    },
  });

  const reconcile = useMutation({
    mutationFn: (repositoryId: string) => reconcileGlobalGitRepository(repositoryId),
    onSuccess: (_data, repositoryId) => invalidateRepo(repositoryId),
  });

  return {
    createRepository,
    deleteRepository,
    createWorktree,
    registerWorktree,
    deleteWorktree,
    associateBranch,
    removeAssociation,
    reconcile,
  };
}
