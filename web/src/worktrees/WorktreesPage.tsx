import { useEffect, useState } from "react";
import { useSearchParams } from "react-router-dom";
import type { GitRepository } from "@/types";
import { Button } from "@/components/ui";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { EmptyState } from "@/shared/EmptyState";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TASK_TIMINGS } from "@/constants/tasks";
import { TaskDraftsListSkeleton } from "@/tasks/components/skeletons";
import { useGlobalRepositories } from "./hooks/useGlobalRepositories";
import { useGlobalGitMutations } from "./hooks/useGlobalGitMutations";
import { RepositoryCard } from "./components/RepositoryCard";
import { DeleteConfirmDialog } from "./components/DeleteConfirmDialog";
import type { GitDeleteTarget } from "./gitDeleteErrors";
import { RegisterRepositoryModal } from "./modals/RegisterRepositoryModal";
import { CreateWorktreeModal } from "./modals/CreateWorktreeModal";
import { AssociateBranchModal } from "./modals/AssociateBranchModal";
import {
  deriveWorktreesPageMode,
  worktreesPageErrorMessage,
  worktreesPageTitle,
} from "./worktreesPageMode";

type ActiveRepoModal =
  | { kind: "worktree"; repository: GitRepository }
  | { kind: "branch"; repository: GitRepository; worktreeId: string }
  | null;

export function WorktreesPage() {
  const repositoriesQuery = useGlobalRepositories();
  const mutations = useGlobalGitMutations();
  const [searchParams, setSearchParams] = useSearchParams();

  const [registerOpen, setRegisterOpen] = useState(false);
  const [activeRepoModal, setActiveRepoModal] = useState<ActiveRepoModal>(null);
  const [deleteTarget, setDeleteTarget] = useState<GitDeleteTarget | null>(null);
  const [deleteError, setDeleteError] = useState<unknown>(null);

  const repositories = repositoriesQuery.data ?? [];
  const pageMode = deriveWorktreesPageMode({
    isLoading: repositoriesQuery.isLoading && !repositoriesQuery.data,
    isError: repositoriesQuery.isError,
    repositoryCount: repositories.length,
  });
  const pageTitle = worktreesPageTitle();
  useDocumentTitle(pageTitle);

  const showSkeleton = useDelayedTrue(
    pageMode === "loading",
    TASK_TIMINGS.draftResumeMinLoadingMs,
  );

  useEffect(() => {
    if (searchParams.get("register") !== "1") return;
    setRegisterOpen(true);
    setSearchParams({}, { replace: true });
  }, [searchParams, setSearchParams]);

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
      } else if (deleteTarget.kind === "worktree") {
        await mutations.deleteWorktree.mutateAsync({
          worktreeId: deleteTarget.id,
          repositoryId: deleteTarget.repositoryId,
        });
      } else {
        await mutations.removeAssociation.mutateAsync({
          worktreeId: deleteTarget.worktreeId,
          branchId: deleteTarget.id,
          repositoryId: deleteTarget.repositoryId,
        });
      }
      closeDelete();
    } catch (err) {
      setDeleteError(err);
    }
  };

  const deletePending =
    mutations.deleteRepository.isPending ||
    mutations.deleteWorktree.isPending ||
    mutations.removeAssociation.isPending;

  return (
    <div className="task-detail-content--enter">
      <section
        className="panel task-list-section-panel worktrees-page"
        aria-labelledby="worktrees-heading"
      >
        <header className="task-list-section-head">
          <div className="task-list-section-head__text">
            <h2 id="worktrees-heading" className="task-list-section-title">
              {pageTitle}
            </h2>
          </div>
          <div className="task-list-section-actions">
            {pageMode === "setup" || pageMode === "manage" ? (
              <Button
                type="button"
                variant="primary"
                className="task-home-new-task-btn"
                onClick={() => setRegisterOpen(true)}
              >
                Register repository
              </Button>
            ) : null}
          </div>
        </header>

        {pageMode === "error" ? (
          <div className="err" role="alert">
            <p>{worktreesPageErrorMessage(repositoriesQuery.error)}</p>
            <div className="task-detail-error-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => {
                  void repositoriesQuery.refetch();
                }}
              >
                Try again
              </button>
            </div>
          </div>
        ) : null}

        <div className="task-list-content task-list-content--enter">
          {showSkeleton ? <TaskDraftsListSkeleton /> : null}
          {pageMode === "setup" ? (
            <div className="task-list-empty-cell">
              <EmptyState
                title="Register a repository to get started"
                description="Hamix needs a git checkout before you can add worktrees, bind branches, and run agent tasks."
                hideIcon
                className="empty-state--in-table empty-state--task-list-fresh"
                action={{
                  label: "Register repository",
                  onClick: () => setRegisterOpen(true),
                }}
              />
            </div>
          ) : null}
          {pageMode === "manage" ? (
            <div className="worktrees-page__cards">
              {repositories.map((repository) => (
                <RepositoryCard
                  key={repository.id}
                  repository={repository}
                  reconcilePending={mutations.reconcile.isPending}
                  onReconcile={() => void mutations.reconcile.mutate(repository.id)}
                  onRegisterWorktree={() =>
                    setActiveRepoModal({ kind: "worktree", repository })
                  }
                  onAssociateBranch={(worktreeId) =>
                    setActiveRepoModal({ kind: "branch", repository, worktreeId })
                  }
                  onDeleteRepository={() =>
                    setDeleteTarget({
                      kind: "repository",
                      id: repository.id,
                      label: repository.path,
                      repositoryId: repository.id,
                    })
                  }
                  onDeleteWorktree={(worktreeId, label) =>
                    setDeleteTarget({
                      kind: "worktree",
                      id: worktreeId,
                      label,
                      repositoryId: repository.id,
                    })
                  }
                  onDeleteAssociation={(assocId, _branchId, worktreeId, label) =>
                    setDeleteTarget({
                      kind: "branch",
                      id: assocId,
                      label,
                      repositoryId: repository.id,
                      worktreeId,
                    })
                  }
                />
              ))}
            </div>
          ) : null}
        </div>
      </section>

      <RegisterRepositoryModal
        open={registerOpen}
        pending={mutations.createRepository.isPending}
        error={mutations.createRepository.error}
        onClose={() => {
          setRegisterOpen(false);
          mutations.createRepository.reset();
        }}
        onSubmit={(input) => {
          void mutations.createRepository
            .mutateAsync(input)
            .then(() => setRegisterOpen(false));
        }}
      />

      <CreateWorktreeModal
        open={activeRepoModal?.kind === "worktree"}
        pending={mutations.createWorktree.isPending}
        error={mutations.createWorktree.error}
        defaultBranch={activeRepoModal?.repository.default_branch}
        onClose={() => {
          setActiveRepoModal(null);
          mutations.createWorktree.reset();
        }}
        onSubmit={(input) => {
          const repo = activeRepoModal?.repository;
          if (!repo) return;
          void mutations.createWorktree
            .mutateAsync({ repositoryId: repo.id, input })
            .then(() => setActiveRepoModal(null));
        }}
      />

      <AssociateBranchModal
        open={activeRepoModal?.kind === "branch"}
        pending={mutations.associateBranch.isPending}
        error={mutations.associateBranch.error}
        repositoryId={
          activeRepoModal?.kind === "branch"
            ? activeRepoModal.repository.id
            : ""
        }
        onClose={() => {
          setActiveRepoModal(null);
          mutations.associateBranch.reset();
        }}
        onSubmit={(input) => {
          const modal = activeRepoModal;
          if (modal?.kind !== "branch") return;
          void mutations.associateBranch
            .mutateAsync({
              worktreeId: modal.worktreeId,
              repositoryId: modal.repository.id,
              input,
            })
            .then(() => setActiveRepoModal(null));
        }}
      />

      <DeleteConfirmDialog
        target={deleteTarget}
        pending={deletePending}
        error={deleteError}
        onClose={closeDelete}
        onConfirm={() => void runDelete()}
      />
    </div>
  );
}
