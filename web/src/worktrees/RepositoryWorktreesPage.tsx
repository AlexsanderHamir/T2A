import { Link, useNavigate, useParams } from "react-router-dom";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { useOptionalToast } from "@/shared/toast";
import { EmptyState } from "@/shared/EmptyState";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TASK_TIMINGS } from "@/constants/tasks";
import { TaskDraftsListSkeleton } from "@/components/skeletons/TaskDraftsListSkeleton";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
} from "./repositoryDisplay";
import { useGlobalRepository } from "./hooks/useGlobalRepository";
import { useRepositoryGitActions } from "./hooks/useRepositoryGitActions";
import { RepositoryWorktreesSection } from "./components/RepositoryWorktreesSection";
import { DeleteConfirmDialog } from "./components/DeleteConfirmDialog";
import { RegisterWorktreeModal } from "./modals/RegisterWorktreeModal";
import { CreateWorktreeModal } from "./modals/CreateWorktreeModal";
import { RelocateRepositoryModal } from "./modals/RelocateRepositoryModal";
import { formatReconcileSuccess } from "./gitReconcileErrors";
import { worktreeGitCopy } from "./worktreeGitCopy";
import { WorktreesFolderIcon, WorktreesMoreIcon } from "./components/WorktreesIcons";
import { WorktreesMenu } from "./components/WorktreesMenu";

export function RepositoryWorktreesPage() {
  const { repositoryId = "" } = useParams();
  const navigate = useNavigate();
  const toast = useOptionalToast();
  const repositoryQuery = useGlobalRepository(repositoryId);
  const repository = repositoryQuery.data;

  const actions = useRepositoryGitActions({
    repository,
    onRepositoryDeleted: () => navigate("/worktrees"),
  });

  const displayName = repository
    ? repositoryDisplayName(repository.path)
    : "Repository";
  useDocumentTitle(repository ? `${displayName} worktrees` : "Repository worktrees");

  const showSkeleton = useDelayedTrue(
    repositoryQuery.isLoading && !repositoryQuery.data,
    TASK_TIMINGS.draftResumeMinLoadingMs,
  );

  if (!repositoryId) {
    return (
      <section className="panel task-list-section-panel task-detail-content--enter repository-detail">
        <EmptyState
          title="Missing repository id"
          description="Choose a repository from the list."
          hideIcon
          className="empty-state--task-list-fresh"
        />
      </section>
    );
  }

  return (
    <section
      className="panel task-list-section-panel task-detail-content--enter worktrees-page repository-detail"
      aria-labelledby="repository-detail-heading"
    >
      <header className="repository-detail__header">
        <Link to="/worktrees" className="repository-detail__back pd__back project-context-back-link">
          <span aria-hidden="true">&#8249;</span>
          All repositories
        </Link>
      </header>

      {repositoryQuery.isError && !repositoryQuery.isLoading ? (
        <div className="err" role="alert">
          <p>
            {repositoryQuery.error instanceof Error
              ? repositoryQuery.error.message
              : "Could not load repository."}
          </p>
          <div className="task-detail-error-actions">
            <button
              type="button"
              className="secondary"
              onClick={() => {
                void repositoryQuery.refetch();
              }}
            >
              Try again
            </button>
          </div>
        </div>
      ) : null}

      {showSkeleton ? <TaskDraftsListSkeleton /> : null}

      {repository ? (
        <>
          <div className="repository-detail__title-block">
            <div className="repository-detail__title-row">
              <h1 id="repository-detail-heading" className="repository-detail__title">
                {displayName}
              </h1>
              <div className="repository-detail__actions">
                <WorktreesMenu
                  triggerLabel={worktreeGitCopy.repositoryActions}
                  className="task-list-icon-btn"
                  icon={<WorktreesMoreIcon />}
                  iconOnly
                  triggerDisabled={actions.reconcilePending}
                  triggerBusy={actions.reconcilePending}
                  items={[
                    {
                      id: "reconcile",
                      label: actions.reconcilePending
                        ? worktreeGitCopy.reconciling
                        : worktreeGitCopy.reconcile,
                      onSelect: () => void actions.handleReconcile(repository),
                      disabled: actions.reconcilePending,
                    },
                    {
                      id: "delete-repository",
                      label: worktreeGitCopy.deleteRepository,
                      onSelect: actions.openDeleteRepository,
                      danger: true,
                    },
                  ]}
                />
              </div>
            </div>
            <p className="repository-detail__path" title={repository.path}>
              <WorktreesFolderIcon className="repository-detail__path-icon" aria-hidden />
              <span>{repository.path}</span>
            </p>
            {repository.host_path.trim() !== "" &&
            !repositoryPathsEquivalent(repository.path, repository.host_path) ? (
              <p className="repository-detail__host-path">
                <span className="worktrees-repo-row__meta-label">
                  {worktreeGitCopy.hostPathLabel}
                </span>
                <code>{repository.host_path}</code>
              </p>
            ) : null}
          </div>

          <div className="task-list-content task-list-content--enter">
            <RepositoryWorktreesSection
              repository={repository}
              reconcilePending={actions.reconcilePending}
              reconcileError={actions.reconcileError}
              onRegisterWorktree={() => actions.setActiveWorktreeModal("register-worktree")}
              onCreateWorktree={() => actions.setActiveWorktreeModal("create-worktree")}
              onDeleteWorktree={actions.openDeleteWorktree}
            />
          </div>
        </>
      ) : null}

      {!repositoryQuery.isLoading && !repositoryQuery.isError && !repository ? (
        <EmptyState
          title="Repository not found"
          description="It may have been removed. Return to the repository list."
          hideIcon
          className="empty-state--task-list-fresh"
        />
      ) : null}

      <RegisterWorktreeModal
        open={actions.activeWorktreeModal === "register-worktree"}
        pending={actions.mutations.registerWorktree.isPending}
        error={actions.mutations.registerWorktree.error}
        repositoryId={repository?.id ?? ""}
        storedPath={repository?.path ?? ""}
        reconcilePending={actions.reconcilePending}
        reconcileError={actions.reconcileError}
        reconcileBlocked={actions.reconcileBlocked}
        onReconcile={() => {
          if (repository != null) void actions.handleReconcile(repository);
        }}
        onClose={() => {
          actions.setActiveWorktreeModal(null);
          actions.mutations.registerWorktree.reset();
        }}
        onSubmit={(input) => {
          if (!repository || actions.activeWorktreeModal !== "register-worktree") return;
          void actions.mutations.registerWorktree
            .mutateAsync({ repositoryId: repository.id, input })
            .then(() => actions.setActiveWorktreeModal(null));
        }}
      />

      <CreateWorktreeModal
        open={actions.activeWorktreeModal === "create-worktree"}
        pending={actions.mutations.createWorktree.isPending}
        error={actions.mutations.createWorktree.error}
        repositoryId={repository?.id ?? ""}
        storedPath={repository?.path ?? ""}
        reconcilePending={actions.reconcilePending}
        reconcileError={actions.reconcileError}
        reconcileBlocked={actions.reconcileBlocked}
        onReconcile={() => {
          if (repository != null) void actions.handleReconcile(repository);
        }}
        onClose={() => {
          actions.setActiveWorktreeModal(null);
          actions.mutations.createWorktree.reset();
        }}
        onSubmit={(input) => {
          if (!repository || actions.activeWorktreeModal !== "create-worktree") return;
          void actions.mutations.createWorktree
            .mutateAsync({ repositoryId: repository.id, input })
            .then(() => actions.setActiveWorktreeModal(null));
        }}
      />

      <DeleteConfirmDialog
        target={actions.deleteTarget}
        pending={actions.deletePending}
        error={actions.deleteError}
        onClose={actions.closeDelete}
        onConfirm={() => void actions.runDelete()}
      />

      <RelocateRepositoryModal
        open={actions.relocateRepository != null}
        pending={actions.mutations.relocateRepository.isPending}
        error={actions.mutations.relocateRepository.error}
        storedPath={actions.relocateRepository?.path ?? ""}
        onClose={actions.closeRelocateModal}
        onSubmit={(input) => {
          const repo = actions.relocateRepository;
          if (!repo) return;
          void actions.mutations.relocateRepository
            .mutateAsync({ repositoryId: repo.id, input })
            .then((result) => {
              actions.setAutoReconcileBlocked((prev) => {
                const next = { ...prev };
                delete next[repo.id];
                return next;
              });
              actions.closeRelocateModal();
              toast?.success(formatReconcileSuccess(result));
            });
        }}
      />
    </section>
  );
}
