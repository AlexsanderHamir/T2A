import { useState } from "react";
import { DEFAULT_PROJECT_ID, type GitBranch, type GitRepository, type GitWorktree } from "@/types";
import { Button } from "@/components/ui";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { EmptyState } from "@/shared/EmptyState";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TASK_TIMINGS } from "@/constants/tasks";
import { TaskDraftsListSkeleton } from "@/tasks/components/skeletons";
import { useRepositories } from "./hooks/useRepositories";
import { useGitMutations } from "./hooks/useGitMutations";
import { RepositoryCard } from "./components/RepositoryCard";
import { DeleteConfirmDialog } from "./components/DeleteConfirmDialog";
import type { GitDeleteTarget } from "./gitDeleteErrors";
import { RegisterRepositoryModal } from "./modals/RegisterRepositoryModal";
import { CreateWorktreeModal } from "./modals/CreateWorktreeModal";
import { CreateBranchModal } from "./modals/CreateBranchModal";

type ActiveRepoModal =
  | { kind: "worktree"; repository: GitRepository }
  | { kind: "branch"; repository: GitRepository }
  | null;

export function WorktreesPage() {
  useDocumentTitle("Worktrees");
  const projectId = DEFAULT_PROJECT_ID;
  const repositoriesQuery = useRepositories(projectId);
  const mutations = useGitMutations(projectId);

  const [registerOpen, setRegisterOpen] = useState(false);
  const [activeRepoModal, setActiveRepoModal] = useState<ActiveRepoModal>(null);
  const [deleteTarget, setDeleteTarget] = useState<GitDeleteTarget | null>(null);
  const [deleteError, setDeleteError] = useState<unknown>(null);

  const showSkeleton = useDelayedTrue(
    repositoriesQuery.isLoading && !repositoriesQuery.data,
    TASK_TIMINGS.draftResumeMinLoadingMs,
  );

  const repositories = repositoriesQuery.data ?? [];

  const closeDelete = () => {
    setDeleteTarget(null);
    setDeleteError(null);
  };

  const runDelete = async () => {
    if (!deleteTarget) return;
    setDeleteError(null);
    try {
      if (deleteTarget.kind === "repository") {
        await mutations.removeRepository.mutateAsync(deleteTarget.id);
      } else if (deleteTarget.kind === "worktree") {
        await mutations.removeWorktree.mutateAsync({
          worktreeId: deleteTarget.id,
          repositoryId: deleteTarget.repositoryId,
        });
      } else {
        await mutations.removeBranch.mutateAsync({
          branchId: deleteTarget.id,
          repositoryId: deleteTarget.repositoryId,
        });
      }
      closeDelete();
    } catch (err) {
      setDeleteError(err);
    }
  };

  return (
    <div className="task-detail-content--enter">
      <section
        className="panel task-list-section-panel worktrees-page"
        aria-labelledby="worktrees-heading"
      >
        <header className="task-list-section-head">
          <div className="task-list-section-head__text">
            <h2 id="worktrees-heading" className="task-list-section-title">
              Worktrees
            </h2>
          </div>
          <div className="task-list-section-actions">
            {!showSkeleton ? (
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

        {repositoriesQuery.isError ? (
          <div className="err" role="alert">
            <p>
              {repositoriesQuery.error instanceof Error
                ? repositoriesQuery.error.message
                : "Could not load repositories"}
            </p>
          </div>
        ) : null}

        <div className="task-list-content task-list-content--enter">
          {showSkeleton ? <TaskDraftsListSkeleton /> : null}
          {!showSkeleton && repositories.length === 0 ? (
            <div className="task-list-empty-cell">
              <EmptyState
                title="No repositories yet"
                description=""
                hideIcon
                className="empty-state--in-table empty-state--task-list-fresh"
              />
            </div>
          ) : null}
          {!showSkeleton && repositories.length > 0 ? (
            <div className="worktrees-page__cards">
              {repositories.map((repository) => (
                <RepositoryCard
                  key={repository.id}
                  projectId={projectId}
                  repository={repository}
                  reconcilePending={mutations.reconcile.isPending}
                  onReconcile={() => void mutations.reconcile.mutate(repository.id)}
                  onRegisterWorktree={() =>
                    setActiveRepoModal({ kind: "worktree", repository })
                  }
                  onRegisterBranch={() =>
                    setActiveRepoModal({ kind: "branch", repository })
                  }
                  onDeleteRepository={() =>
                    setDeleteTarget({
                      kind: "repository",
                      id: repository.id,
                      label: repository.path,
                      repositoryId: repository.id,
                    })
                  }
                  onDeleteWorktree={(worktree: GitWorktree) =>
                    setDeleteTarget({
                      kind: "worktree",
                      id: worktree.id,
                      label: worktree.name || worktree.path,
                      repositoryId: repository.id,
                    })
                  }
                  onDeleteBranch={(branch: GitBranch) =>
                    setDeleteTarget({
                      kind: "branch",
                      id: branch.id,
                      label: branch.name,
                      repositoryId: repository.id,
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
        pending={mutations.registerRepository.isPending}
        error={mutations.registerRepository.error}
        onClose={() => {
          setRegisterOpen(false);
          mutations.registerRepository.reset();
        }}
        onSubmit={(input) => {
          void mutations.registerRepository
            .mutateAsync(input)
            .then(() => setRegisterOpen(false));
        }}
      />

      <CreateWorktreeModal
        open={activeRepoModal?.kind === "worktree"}
        pending={mutations.addWorktree.isPending}
        error={mutations.addWorktree.error}
        defaultBranch={activeRepoModal?.repository.default_branch}
        onClose={() => {
          setActiveRepoModal(null);
          mutations.addWorktree.reset();
        }}
        onSubmit={(input) => {
          const repo = activeRepoModal?.repository;
          if (!repo) return;
          void mutations.addWorktree
            .mutateAsync({ repositoryId: repo.id, ...input })
            .then(() => setActiveRepoModal(null));
        }}
      />

      <CreateBranchModal
        open={activeRepoModal?.kind === "branch"}
        pending={mutations.addBranch.isPending}
        error={mutations.addBranch.error}
        onClose={() => {
          setActiveRepoModal(null);
          mutations.addBranch.reset();
        }}
        onSubmit={(input) => {
          const repo = activeRepoModal?.repository;
          if (!repo) return;
          void mutations.addBranch
            .mutateAsync({ repositoryId: repo.id, ...input })
            .then(() => setActiveRepoModal(null));
        }}
      />

      <DeleteConfirmDialog
        target={deleteTarget}
        pending={mutations.removeRepository.isPending || mutations.removeWorktree.isPending || mutations.removeBranch.isPending}
        error={deleteError}
        onClose={closeDelete}
        onConfirm={() => void runDelete()}
      />
    </div>
  );
}
