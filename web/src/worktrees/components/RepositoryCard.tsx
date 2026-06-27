import type { GitRepository } from "@/types/git";
import { EmptyState } from "@/shared/EmptyState";
import { useGlobalBranches } from "../hooks/useGlobalBranches";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
} from "../repositoryDisplay";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { WorktreeList } from "./WorktreeList";

type Props = {
  repository: GitRepository;
  onRegisterWorktree: () => void;
  onCreateWorktree: () => void;
  onDeleteRepository: () => void;
  onDeleteWorktree: (worktreeId: string, label: string) => void;
  onReconcile: () => void;
  reconcilePending?: boolean;
};

export function RepositoryCard({
  repository,
  onRegisterWorktree,
  onCreateWorktree,
  onDeleteRepository,
  onDeleteWorktree,
  onReconcile,
  reconcilePending = false,
}: Props) {
  const worktreesQuery = useGlobalWorktrees(repository.id);
  const branchesQuery = useGlobalBranches(repository.id);
  const worktrees = worktreesQuery.data ?? [];
  const branches = branchesQuery.data ?? [];
  const loading = worktreesQuery.isLoading || branchesQuery.isLoading;
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
          <p className="worktrees-repo-card__path" title={repository.path}>
            <code>{repository.path}</code>
          </p>
          {showHostPath ? (
            <p className="worktrees-repo-card__host-path">
              <span className="worktrees-repo-card__meta-label">{worktreeGitCopy.hostPathLabel}</span>
              <code>{repository.host_path}</code>
            </p>
          ) : null}
          {repository.default_branch.trim() !== "" ? (
            <p className="worktrees-repo-card__default-branch">
              <span className="worktrees-repo-card__meta-label">{worktreeGitCopy.defaultBranchLabel}</span>
              <code>{repository.default_branch}</code>
            </p>
          ) : null}
        </div>
        <div className="worktrees-repo-card__header-actions">
          <button
            type="button"
            className="secondary"
            disabled={reconcilePending}
            onClick={onReconcile}
          >
            {reconcilePending ? worktreeGitCopy.reconciling : worktreeGitCopy.reconcile}
          </button>
          <button type="button" className="secondary danger" onClick={onDeleteRepository}>
            {worktreeGitCopy.deleteRepository}
          </button>
        </div>
      </header>

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
          </h3>
          <div className="worktrees-repo-card__section-actions">
            <button type="button" className="secondary" onClick={onRegisterWorktree}>
              {worktreeGitCopy.registerWorktree}
            </button>
            <button type="button" className="secondary" onClick={onCreateWorktree}>
              {worktreeGitCopy.createWorktree}
            </button>
          </div>
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
