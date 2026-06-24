import type { GitRepository } from "@/types/git";
import { useGlobalBranches } from "../hooks/useGlobalBranches";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import { WorktreeRow } from "./WorktreeRow";

type Props = {
  repository: GitRepository;
  onRegisterWorktree: () => void;
  onAssociateBranch: (worktreeId: string) => void;
  onDeleteRepository: () => void;
  onDeleteWorktree: (worktreeId: string, label: string) => void;
  onDeleteAssociation: (
    assocId: string,
    branchId: string,
    worktreeId: string,
    label: string,
  ) => void;
  onReconcile: () => void;
  reconcilePending?: boolean;
};

export function RepositoryCard({
  repository,
  onRegisterWorktree,
  onAssociateBranch,
  onDeleteRepository,
  onDeleteWorktree,
  onDeleteAssociation,
  onReconcile,
  reconcilePending = false,
}: Props) {
  const worktreesQuery = useGlobalWorktrees(repository.id);
  const branchesQuery = useGlobalBranches(repository.id);
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

      <section
        className="worktrees-repo-card__section"
        aria-labelledby={`repo-${repository.id}-worktrees`}
      >
        <div className="worktrees-repo-card__section-header">
          <h3
            id={`repo-${repository.id}-worktrees`}
            className="worktrees-repo-card__section-title"
          >
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
                onDelete={() =>
                  onDeleteWorktree(
                    worktree.id,
                    worktree.name.trim() || worktree.path,
                  )
                }
                onAssociateBranch={() => onAssociateBranch(worktree.id)}
                onDeleteAssociation={(assocId, branchId, label) =>
                  onDeleteAssociation(assocId, branchId, worktree.id, label)
                }
              />
            ))}
          </div>
        )}
      </section>
    </article>
  );
}
