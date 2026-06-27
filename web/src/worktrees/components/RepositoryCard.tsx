import type { GitRepository } from "@/types/git";
import { EmptyState } from "@/shared/EmptyState";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { useGlobalBranches } from "../hooks/useGlobalBranches";
import { useGlobalLiveWorktrees } from "../hooks/useGlobalLiveWorktrees";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
} from "../repositoryDisplay";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { gitReconcileErrorMessage } from "../gitReconcileErrors";
import {
  WorktreesMoreIcon,
  WorktreesPlusIcon,
  WorktreesRefreshIcon,
} from "./WorktreesIcons";
import { WorktreesMenu } from "./WorktreesMenu";
import { WorktreesPathChip } from "./WorktreesPathChip";
import { WorktreeList } from "./WorktreeList";

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
  const liveWorktreesQuery = useGlobalLiveWorktrees(repository.id);
  const branchesQuery = useGlobalBranches(repository.id);
  const worktrees = worktreesQuery.data ?? [];
  const liveWorktrees = liveWorktreesQuery.data ?? [];
  const branches = branchesQuery.data ?? [];
  const loading = worktreesQuery.isLoading || branchesQuery.isLoading;
  const unregisteredLiveCount = liveWorktrees.filter((wt) => !wt.registered).length;
  const reconcileErrorMessage =
    reconcileError != null ? gitReconcileErrorMessage(reconcileError) : null;
  const repoName = repositoryDisplayName(repository.path);
  const showHostPath =
    repository.host_path.trim() !== "" &&
    !repositoryPathsEquivalent(repository.path, repository.host_path);

  return (
    <article className="worktrees-repo-card" aria-labelledby={`repo-${repository.id}-title`}>
      <header className="worktrees-repo-card__header">
        <div className="worktrees-repo-card__heading">
          <h2 id={`repo-${repository.id}-title`} className="worktrees-repo-card__title">
            {repoName}
          </h2>
          <div className="worktrees-repo-card__meta-row">
            <WorktreesPathChip path={repository.path} />
          </div>
          {showHostPath ? (
            <p className="worktrees-repo-card__host-path">
              <span className="worktrees-repo-card__meta-label">{worktreeGitCopy.hostPathLabel}</span>
              <code>{repository.host_path}</code>
            </p>
          ) : null}
        </div>
        <div className="worktrees-repo-card__header-actions">
          <button
            type="button"
            className="secondary worktrees-toolbar-btn"
            disabled={reconcilePending}
            onClick={onReconcile}
          >
            <WorktreesRefreshIcon className="worktrees-toolbar-btn__icon" />
            {reconcilePending ? worktreeGitCopy.reconciling : worktreeGitCopy.reconcile}
          </button>
          <WorktreesMenu
            triggerLabel={worktreeGitCopy.repositoryActions}
            className="secondary worktrees-icon-menu-btn"
            icon={<WorktreesMoreIcon />}
            iconOnly
            items={[
              {
                id: "delete-repository",
                label: worktreeGitCopy.deleteRepository,
                onSelect: onDeleteRepository,
                danger: true,
              },
            ]}
          />
        </div>
      </header>

      {unregisteredLiveCount > 0 ? (
        <div className="worktrees-repo-card__drift-banner" role="status">
          <p className="worktrees-repo-card__drift-title">{worktreeGitCopy.driftBannerTitle}</p>
          <p className="worktrees-repo-card__drift-description">
            {worktreeGitCopy.driftBannerDescription}
          </p>
        </div>
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
        {loading ? (
          <p className="worktrees-repo-card__loading" aria-busy="true">
            Loading worktrees…
          </p>
        ) : worktrees.length === 0 ? (
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
