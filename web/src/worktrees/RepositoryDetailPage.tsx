import { useState } from "react";
import { Link, useNavigate, useParams } from "react-router-dom";
import { Button } from "@/components/ui";
import { EmptyState } from "@/shared/EmptyState";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useGlobalRepository } from "./hooks/useGlobalRepository";
import { useGlobalGitMutations } from "./hooks/useGlobalGitMutations";
import { RepositoryWorktreesPanel } from "./components/RepositoryWorktreesPanel";
import { DeleteConfirmDialog } from "./components/DeleteConfirmDialog";
import type { GitDeleteTarget } from "./gitDeleteErrors";
import { RegisterWorktreeModal } from "./modals/RegisterWorktreeModal";
import { CreateWorktreeModal } from "./modals/CreateWorktreeModal";
import { AssociateBranchModal } from "./modals/AssociateBranchModal";
import { repositoryDisplayName } from "./repositoryDisplay";

type ActiveModal =
  | { kind: "register-worktree" }
  | { kind: "create-worktree" }
  | { kind: "branch"; worktreeId: string }
  | null;

export function RepositoryDetailPage() {
  const { repositoryId = "" } = useParams();
  const navigate = useNavigate();
  const repositoryQuery = useGlobalRepository(repositoryId);
  const mutations = useGlobalGitMutations();
  const repository = repositoryQuery.data;

  const [activeModal, setActiveModal] = useState<ActiveModal>(null);
  const [deleteTarget, setDeleteTarget] = useState<GitDeleteTarget | null>(null);
  const [deleteError, setDeleteError] = useState<unknown>(null);

  const title = repository
    ? `${repositoryDisplayName(repository.path)} repository`
    : "Repository";
  useDocumentTitle(title);

  const closeDelete = () => {
    setDeleteTarget(null);
    setDeleteError(null);
  };

  const runDelete = async () => {
    if (!deleteTarget || !repositoryId) return;
    setDeleteError(null);
    try {
      if (deleteTarget.kind === "repository") {
        await mutations.deleteRepository.mutateAsync(deleteTarget.id);
        navigate("/worktrees");
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

  if (!repositoryId) {
    return (
      <section className="panel task-detail-panel">
        <EmptyState
          title="Missing repository id"
          description="Choose a repository from the repositories list."
          density="compact"
          hideIcon
        />
      </section>
    );
  }

  const hostPath = repository?.host_path.trim() ?? "";
  const showHostPath =
    Boolean(repository) && hostPath !== "" && hostPath !== repository.path;

  return (
    <div className="task-detail-content--enter">
      <section className="panel task-detail-panel worktrees-detail">
        <header className="pd__header worktrees-detail__header">
          <Link to="/worktrees" className="pd__back project-context-back-link">
            <span aria-hidden="true">&#8249;</span>
            All repositories
          </Link>
          {repository ? (
            <div className="worktrees-detail__header-actions">
              <Button
                type="button"
                variant="secondary"
                disabled={mutations.reconcile.isPending}
                onClick={() => void mutations.reconcile.mutate(repository.id)}
              >
                {mutations.reconcile.isPending ? "Reconciling…" : "Reconcile"}
              </Button>
              <Button
                type="button"
                variant="secondary"
                className="worktrees-detail__delete-repo"
                onClick={() =>
                  setDeleteTarget({
                    kind: "repository",
                    id: repository.id,
                    label: repository.path,
                    repositoryId: repository.id,
                  })
                }
              >
                Delete repository
              </Button>
            </div>
          ) : null}
        </header>

        {repositoryQuery.isLoading ? (
          <p className="worktrees-detail__loading" aria-busy="true">
            Loading repository…
          </p>
        ) : null}

        {repositoryQuery.error ? (
          <div className="pd__error" role="alert">
            <div className="pd__error-dot" aria-hidden="true" />
            <div>
              <p className="pd__error-title">Unable to load this repository</p>
              <p className="pd__error-message">{repositoryQuery.error.message}</p>
            </div>
          </div>
        ) : null}

        {repository ? (
          <div className="worktrees-detail__body">
            <div className="pd__title-block">
              <h1 className="pd__title">{repositoryDisplayName(repository.path)}</h1>
              <p className="pd__subtitle worktrees-detail__path" title={repository.path}>
                {repository.path}
              </p>
              {showHostPath ? (
                <p className="pd__subtitle worktrees-detail__host-path">
                  Host path: {hostPath}
                </p>
              ) : null}
            </div>

            <RepositoryWorktreesPanel
              repositoryId={repository.id}
              onRegisterWorktree={() => setActiveModal({ kind: "register-worktree" })}
              onCreateWorktree={() => setActiveModal({ kind: "create-worktree" })}
              onAssociateBranch={(worktreeId) =>
                setActiveModal({ kind: "branch", worktreeId })
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
          </div>
        ) : null}
      </section>

      <RegisterWorktreeModal
        open={activeModal?.kind === "register-worktree"}
        pending={mutations.registerWorktree.isPending}
        error={mutations.registerWorktree.error}
        repositoryId={repositoryId}
        onClose={() => {
          setActiveModal(null);
          mutations.registerWorktree.reset();
        }}
        onSubmit={(input) => {
          void mutations.registerWorktree
            .mutateAsync({ repositoryId, input })
            .then(() => setActiveModal(null));
        }}
      />

      <CreateWorktreeModal
        open={activeModal?.kind === "create-worktree"}
        pending={mutations.createWorktree.isPending}
        error={mutations.createWorktree.error}
        repositoryId={repositoryId}
        onClose={() => {
          setActiveModal(null);
          mutations.createWorktree.reset();
        }}
        onSubmit={(input) => {
          void mutations.createWorktree
            .mutateAsync({ repositoryId, input })
            .then(() => setActiveModal(null));
        }}
      />

      <AssociateBranchModal
        open={activeModal?.kind === "branch"}
        pending={mutations.associateBranch.isPending}
        error={mutations.associateBranch.error}
        repositoryId={repositoryId}
        onClose={() => {
          setActiveModal(null);
          mutations.associateBranch.reset();
        }}
        onSubmit={(input) => {
          const modal = activeModal;
          if (modal?.kind !== "branch") return;
          void mutations.associateBranch
            .mutateAsync({
              worktreeId: modal.worktreeId,
              repositoryId,
              input,
            })
            .then(() => setActiveModal(null));
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
