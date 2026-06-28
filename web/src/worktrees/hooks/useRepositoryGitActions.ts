import { useCallback, useState } from "react";
import type { GitRepository } from "@/types";
import { useOptionalToast } from "@/shared/toast";
import type { GitDeleteTarget } from "../gitDeleteErrors";
import { formatReconcileSuccess } from "../gitReconcileErrors";
import { useGlobalGitMutations } from "./useGlobalGitMutations";

type ActiveWorktreeModal = "register-worktree" | "create-worktree" | null;

type Options = {
  repository: GitRepository | null | undefined;
  onRepositoryDeleted?: () => void;
};

export function useRepositoryGitActions({ repository, onRepositoryDeleted }: Options) {
  const mutations = useGlobalGitMutations();
  const toast = useOptionalToast();

  const [activeWorktreeModal, setActiveWorktreeModal] = useState<ActiveWorktreeModal>(null);
  const [deleteTarget, setDeleteTarget] = useState<GitDeleteTarget | null>(null);
  const [deleteError, setDeleteError] = useState<unknown>(null);
  const [relocateRepository, setRelocateRepository] = useState<GitRepository | null>(null);
  const [reconcileErrors, setReconcileErrors] = useState<Record<string, unknown>>({});
  const [autoReconcileBlocked, setAutoReconcileBlocked] = useState<Record<string, true>>({});

  const closeDelete = () => {
    setDeleteTarget(null);
    setDeleteError(null);
  };

  const runDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError(null);
    try {
      if (deleteTarget.kind === "repository") {
        await mutations.deleteRepository.mutateAsync(deleteTarget.id);
        closeDelete();
        onRepositoryDeleted?.();
      } else {
        await mutations.unregisterWorktree.mutateAsync({
          worktreeId: deleteTarget.id,
          repositoryId: deleteTarget.repositoryId,
        });
        closeDelete();
      }
    } catch (err) {
      setDeleteError(err);
    }
  };

  const deletePending =
    mutations.deleteRepository.isPending || mutations.unregisterWorktree.isPending;

  const reconcilingRepositoryId =
    mutations.reconcile.isPending || mutations.relocateRepository.isPending
      ? mutations.reconcile.variables?.repositoryId ??
        mutations.relocateRepository.variables?.repositoryId
      : undefined;

  const handleReconcile = useCallback(
    async (repo: GitRepository) => {
      setReconcileErrors((prev) => {
        const next = { ...prev };
        delete next[repo.id];
        return next;
      });
      try {
        const result = await mutations.reconcile.mutateAsync({
          repositoryId: repo.id,
          input: { repair: true },
        });
        if (result.status === "needs_bootstrap_path") {
          setAutoReconcileBlocked((prev) => ({ ...prev, [repo.id]: true }));
          setRelocateRepository(repo);
          return;
        }
        toast?.success(formatReconcileSuccess(result));
      } catch (err) {
        setReconcileErrors((prev) => ({ ...prev, [repo.id]: err }));
      }
    },
    [mutations.reconcile, toast],
  );

  const closeRelocateModal = () => {
    setRelocateRepository(null);
    mutations.relocateRepository.reset();
  };

  const reconcilePending = repository != null && reconcilingRepositoryId === repository.id;
  const reconcileError =
    repository != null ? reconcileErrors[repository.id] : undefined;
  const reconcileBlocked =
    repository != null && autoReconcileBlocked[repository.id] === true;

  const openDeleteRepository = () => {
    if (!repository) return;
    setDeleteTarget({
      kind: "repository",
      id: repository.id,
      label: repository.path,
      repositoryId: repository.id,
    });
  };

  const openDeleteWorktree = (worktreeId: string, label: string) => {
    if (!repository) return;
    setDeleteTarget({
      kind: "worktree",
      id: worktreeId,
      label,
      repositoryId: repository.id,
    });
  };

  return {
    mutations,
    activeWorktreeModal,
    setActiveWorktreeModal,
    deleteTarget,
    deleteError,
    deletePending,
    relocateRepository,
    reconcilePending,
    reconcileError,
    reconcileBlocked,
    closeDelete,
    runDelete,
    closeRelocateModal,
    handleReconcile,
    openDeleteRepository,
    openDeleteWorktree,
    setAutoReconcileBlocked,
  };
}
