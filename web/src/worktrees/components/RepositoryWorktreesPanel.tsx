import { Button } from "@/components/ui";
import { EmptyState } from "@/shared/EmptyState";
import { TaskDraftsListSkeleton } from "@/components/skeletons/TaskDraftsListSkeleton";
import { useGlobalBranches } from "../hooks/useGlobalBranches";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import { WorktreeRow } from "./WorktreeRow";

type Props = {
  repositoryId: string;
  onRegisterWorktree: () => void;
  onCreateWorktree: () => void;
  onAssociateBranch: (worktreeId: string) => void;
  onDeleteWorktree: (worktreeId: string, label: string) => void;
  onDeleteAssociation: (
    assocId: string,
    branchId: string,
    worktreeId: string,
    label: string,
  ) => void;
};

export function RepositoryWorktreesPanel({
  repositoryId,
  onRegisterWorktree,
  onCreateWorktree,
  onAssociateBranch,
  onDeleteWorktree,
  onDeleteAssociation,
}: Props) {
  const worktreesQuery = useGlobalWorktrees(repositoryId);
  const branchesQuery = useGlobalBranches(repositoryId);
  const worktrees = worktreesQuery.data ?? [];
  const branches = branchesQuery.data ?? [];
  const loading = worktreesQuery.isLoading || branchesQuery.isLoading;

  return (
    <section
      className="panel task-list-section-panel worktrees-detail__list"
      aria-labelledby="repository-worktrees-heading"
    >
      <header className="task-list-section-head">
        <div className="task-list-section-head__text">
          <h2 id="repository-worktrees-heading" className="task-list-section-title">
            Worktrees
          </h2>
          <p className="wl__subtitle">
            Linked checkouts and branch associations for agent tasks.
          </p>
        </div>
        <div className="task-list-section-actions worktrees-detail__list-actions">
          <Button type="button" variant="secondary" onClick={onRegisterWorktree}>
            Register worktree
          </Button>
          <Button type="button" variant="secondary" onClick={onCreateWorktree}>
            Create worktree
          </Button>
        </div>
      </header>

      <div className="task-list-content task-list-content--enter">
        {loading ? <TaskDraftsListSkeleton /> : null}
        {!loading && worktrees.length === 0 ? (
          <div className="task-list-empty-cell">
            <EmptyState
              title="No worktrees yet"
              description="Register an existing linked directory or create a new one."
              hideIcon
              className="empty-state--in-table empty-state--task-list-fresh"
            />
          </div>
        ) : null}
        {!loading && worktrees.length > 0 ? (
          <div className="wt__list" aria-label="Worktrees">
            {worktrees.map((worktree, index) => (
              <WorktreeRow
                key={worktree.id}
                worktree={worktree}
                branches={branches}
                index={index}
                onDelete={() =>
                  onDeleteWorktree(worktree.id, worktree.name.trim() || worktree.path)
                }
                onAssociateBranch={() => onAssociateBranch(worktree.id)}
                onDeleteAssociation={(assocId, branchId, label) =>
                  onDeleteAssociation(assocId, branchId, worktree.id, label)
                }
              />
            ))}
          </div>
        ) : null}
      </div>
    </section>
  );
}
