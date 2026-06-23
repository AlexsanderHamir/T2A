import type { GitBranch, GitRepository, GitWorktree } from "@/types/git";
import { useBranches } from "../hooks/useBranches";
import { useWorktrees } from "../hooks/useWorktrees";
import { WorktreeRow } from "./WorktreeRow";
import { BranchPill } from "./BranchPill";

type Props = {
  projectId: string;
  repository: GitRepository;
  onRegisterWorktree: () => void;
  onRegisterBranch: () => void;
  onDeleteRepository: () => void;
  onDeleteWorktree: (worktree: GitWorktree) => void;
  onDeleteBranch: (branch: GitBranch) => void;
  onReconcile: () => void;
  reconcilePending?: boolean;
};

export function RepositoryCard({
  projectId,
  repository,
  onRegisterWorktree,
  onRegisterBranch,
  onDeleteRepository,
  onDeleteWorktree,
  onDeleteBranch,
  onReconcile,
  reconcilePending = false,
}: Props) {
  const worktreesQuery = useWorktrees(projectId, repository.id);
  const branchesQuery = useBranches(projectId, repository.id);
  const worktrees = worktreesQuery.data ?? [];
  const branches = branchesQuery.data ?? [];
  const loading = worktreesQuery.isLoading || branchesQuery.isLoading;

  return (
    <article className="worktrees-repo-card" aria-labelledby={`repo-${repository.id}-title`}>
      <header className="worktrees-repo-card__header">
        <div className="worktrees-repo-card__heading">
          <h2 id={`repo-${repository.id}-title`} className="worktrees-repo-card__title">
            Repository
          </h2>
          <p className="worktrees-repo-card__path" title={repository.path}>
            <code>{repository.path}</code>
          </p>
          {repository.host_path.trim() !== "" ? (
            <p className="worktrees-repo-card__host-path">
              Host path: <code>{repository.host_path}</code>
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
            {reconcilePending ? "Reconciling…" : "Reconcile"}
          </button>
          <button type="button" className="secondary danger" onClick={onDeleteRepository}>
            Delete repository
          </button>
        </div>
      </header>

      <section className="worktrees-repo-card__section" aria-labelledby={`repo-${repository.id}-worktrees`}>
        <div className="worktrees-repo-card__section-header">
          <h3 id={`repo-${repository.id}-worktrees`} className="worktrees-repo-card__section-title">
            Worktrees
          </h3>
          <button type="button" className="secondary" onClick={onRegisterWorktree}>
            Add worktree
          </button>
        </div>
        {loading ? (
          <p className="worktrees-repo-card__loading" aria-busy="true">
            Loading worktrees…
          </p>
        ) : worktrees.length === 0 ? (
          <p className="worktrees-repo-card__empty">No worktrees yet.</p>
        ) : (
          <div className="worktrees-repo-card__rows">
            {worktrees.map((worktree) => (
              <WorktreeRow
                key={worktree.id}
                worktree={worktree}
                branches={branches}
                onDelete={() => onDeleteWorktree(worktree)}
              />
            ))}
          </div>
        )}
      </section>

      <section className="worktrees-repo-card__section" aria-labelledby={`repo-${repository.id}-branches`}>
        <div className="worktrees-repo-card__section-header">
          <h3 id={`repo-${repository.id}-branches`} className="worktrees-repo-card__section-title">
            Branches
          </h3>
          <button type="button" className="secondary" onClick={onRegisterBranch}>
            Add branch
          </button>
        </div>
        {loading ? (
          <p className="worktrees-repo-card__loading" aria-busy="true">
            Loading branches…
          </p>
        ) : branches.length === 0 ? (
          <p className="worktrees-repo-card__empty">No branches yet.</p>
        ) : (
          <ul className="worktrees-branch-list">
            {branches.map((branch) => (
              <li key={branch.id} className="worktrees-branch-list__item">
                <BranchPill branch={branch} />
                <button
                  type="button"
                  className="secondary worktrees-branch-list__delete"
                  onClick={() => onDeleteBranch(branch)}
                >
                  Delete
                </button>
              </li>
            ))}
          </ul>
        )}
      </section>
    </article>
  );
}
