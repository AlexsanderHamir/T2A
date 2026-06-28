import type { GitRepository } from "@/types/git";
import { EmptyState } from "@/shared/EmptyState";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { useGlobalBranches } from "../hooks/useGlobalBranches";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
} from "../repositoryDisplay";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { isLinkedWorktreeForDisplay } from "../worktreeRegistration";
import { gitReconcileErrorMessage } from "../gitReconcileErrors";
import {
  WorktreesFolderIcon,
  WorktreesMoreIcon,
  WorktreesPlusIcon,
} from "./WorktreesIcons";
import { WorktreesMenu } from "./WorktreesMenu";
import { WorktreeList } from "./WorktreeList";
import { WorktreeReconcileStatus } from "./WorktreeReconcileStatus";

type Props = {
  repository: GitRepository;
  onRegisterWorktree: () => void;
  onCreateWorktree: () => void;
  onDeleteRepository: () => void;
  onDeleteWorktree: (worktreeId: string, label: string) => void;
  onReconcile: () => void;
  reconcilePending?: boolean;
  reconcileError?: unknown;
};

export function RepositoryCard({
  repository,
  onRegisterWorktree,
  onCreateWorktree,
  onDeleteRepository,
  onDeleteWorktree,
  onReconcile,
  reconcilePending = false,
  reconcileError,
}: Props) {
  const worktreesQuery = useGlobalWorktrees(repository.id);
  const branchesQuery = useGlobalBranches(repository.id);
  const worktrees = (worktreesQuery.data ?? []).filter(isLinkedWorktreeForDisplay);
  const branches = branchesQuery.data ?? [];
  const loading = worktreesQuery.isLoading || branchesQuery.isLoading;
  const worktreesError =
    worktreesQuery.isError && !worktreesQuery.isLoading
      ? worktreesQuery.error instanceof Error
        ? worktreesQuery.error.message
        : "Could not load worktrees."
      : null;
  const reconcileErrorMessage =
    reconcileError != null ? gitReconcileErrorMessage(reconcileError) : null;
  const repoName = repositoryDisplayName(repository.path);
  const showHostPath =
    repository.host_path.trim() !== "" &&
    !repositoryPathsEquivalent(repository.path, repository.host_path);

  return (
    <article
      className="worktrees-repo-card"
      aria-labelledby={`repo-${repository.id}-title`}
      aria-busy={reconcilePending || undefined}
    >
      <header className="worktrees-repo-card__header">
        <div className="worktrees-repo-card__title-line">
          <h2 id={`repo-${repository.id}-title`} className="worktrees-repo-card__title">
            {repoName}
          </h2>
          <div className="worktrees-repo-card__header-actions">
            <WorktreesMenu
              triggerLabel={worktreeGitCopy.repositoryActions}
              className="secondary worktrees-icon-menu-btn"
              icon={<WorktreesMoreIcon />}
              iconOnly
              triggerDisabled={reconcilePending}
              triggerBusy={reconcilePending}
              items={[
                {
                  id: "reconcile",
                  label: reconcilePending ? worktreeGitCopy.reconciling : worktreeGitCopy.reconcile,
                  onSelect: onReconcile,
                  disabled: reconcilePending,
                },
                {
                  id: "delete-repository",
                  label: worktreeGitCopy.deleteRepository,
                  onSelect: onDeleteRepository,
                  danger: true,
                },
              ]}
            />
          </div>
        </div>
        <div className="worktrees-repo-card__heading-meta">
          <p className="worktrees-repo-card__path" title={repository.path}>
            <WorktreesFolderIcon className="worktrees-repo-card__path-icon" aria-hidden />
            <span className="worktrees-repo-card__path-text">{repository.path}</span>
          </p>
          {showHostPath ? (
            <p className="worktrees-repo-card__host-path">
              <span className="worktrees-repo-card__meta-label">{worktreeGitCopy.hostPathLabel}</span>
              <code>{repository.host_path}</code>
            </p>
          ) : null}
        </div>
      </header>

      {reconcilePending ? (
        <WorktreeReconcileStatus className="worktrees-repo-card__reconcile-status" />
      ) : null}

      {reconcileErrorMessage ? (
        <MutationErrorBanner
          error={reconcileErrorMessage}
          className="worktrees-repo-card__reconcile-error"
        />
      ) : null}

      <section
        className="worktrees-repo-card__section"
        aria-labelledby={`repo-${repository.id}-worktrees`}
      >
        <header className="worktrees-repo-card__section-head">
          <h3
            id={`repo-${repository.id}-worktrees`}
            className="worktrees-repo-card__section-title"
          >
            {worktreeGitCopy.sectionTitle}
            {!loading && worktrees.length > 0 ? (
              <span className="worktrees-section-count" aria-label={`${worktrees.length} worktrees`}>
                {worktrees.length}
              </span>
            ) : null}
          </h3>
          <WorktreesMenu
            triggerLabel={worktreeGitCopy.addWorktree}
            className="secondary worktrees-add-worktree-btn"
            icon={<WorktreesPlusIcon className="worktrees-toolbar-btn__icon" />}
            chevron
            items={[
              {
                id: "register-worktree",
                label: worktreeGitCopy.registerWorktree,
                onSelect: onRegisterWorktree,
              },
              {
                id: "create-worktree",
                label: worktreeGitCopy.createWorktree,
                onSelect: onCreateWorktree,
              },
            ]}
          />
        </header>
        {worktreesError ? (
          <MutationErrorBanner
            error={worktreesError}
            className="worktrees-repo-card__worktrees-error"
          />
        ) : null}
        {loading ? (
          <p className="worktrees-repo-card__loading" aria-busy="true">
            Loading worktrees…
          </p>
        ) : worktreesError ? null : worktrees.length === 0 ? (
          <EmptyState
            title={worktreeGitCopy.emptyWorktreesTitle}
            description={worktreeGitCopy.emptyWorktreesDescription}
            hideIcon
            className="empty-state--in-table empty-state--task-list-fresh"
          />
        ) : (
          <WorktreeList
            worktrees={worktrees}
            branches={branches}
            onDeleteWorktree={onDeleteWorktree}
          />
        )}
      </section>
    </article>
  );
}
