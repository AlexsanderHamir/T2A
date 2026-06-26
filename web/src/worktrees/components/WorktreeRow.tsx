import type { GitBranch, GitWorktree } from "@/types/git";
import { Button } from "@/components/ui";
import { useWorktreeBranchAssociations } from "../hooks/useWorktreeBranchAssociations";
import { BranchPill } from "./BranchPill";

type Props = {
  worktree: GitWorktree;
  branches: GitBranch[];
  index?: number;
  onDelete: () => void;
  deleteDisabled?: boolean;
  onAssociateBranch?: () => void;
  onDeleteAssociation?: (assocId: string, branchId: string, label: string) => void;
};

export function WorktreeRow({
  worktree,
  branches,
  index = 0,
  onDelete,
  deleteDisabled = false,
  onAssociateBranch,
  onDeleteAssociation,
}: Props) {
  const displayName = worktree.name.trim() || worktree.path;
  const hostHint = worktree.path;

  const associationsQuery = useWorktreeBranchAssociations(worktree.id);
  const associations = associationsQuery.data ?? [];

  const branchById = new Map(branches.map((b) => [b.id, b]));

  return (
    <article
      className="wt__row"
      data-main={worktree.is_main ? "true" : "false"}
      style={{ animationDelay: `${index * 40}ms` }}
      aria-label={`Worktree ${displayName}`}
    >
      <div
        className={worktree.is_main ? "wt__row-marker wt__row-marker--main" : "wt__row-marker"}
        aria-hidden="true"
      />

      <div className="wt__row-main">
        <h3 className="wt__row-name">{displayName}</h3>
        <p className="wt__row-desc" title={hostHint}>
          {hostHint}
        </p>
        {worktree.is_main ? (
          <span className="wt__row-badge">Main worktree</span>
        ) : null}
      </div>

      <div className="wt__row-branches" aria-label="Associated branches">
        {associationsQuery.isLoading ? (
          <span className="wt__row-muted">Loading branches…</span>
        ) : associations.length === 0 ? (
          <span className="wt__row-muted">No branches</span>
        ) : (
          <ul className="wt__branch-list">
            {associations.map((assoc) => {
              const branch = branchById.get(assoc.branch_id);
              const label = branch?.name ?? assoc.branch_id;
              return (
                <li key={assoc.id} className="wt__branch-item">
                  {branch ? <BranchPill branch={branch} /> : (
                    <span className="wt__row-muted">{label}</span>
                  )}
                  {onDeleteAssociation ? (
                    <button
                      type="button"
                      className="wt__branch-remove"
                      aria-label={`Remove branch ${label}`}
                      onClick={() => onDeleteAssociation(assoc.id, assoc.branch_id, label)}
                    >
                      Remove
                    </button>
                  ) : null}
                </li>
              );
            })}
          </ul>
        )}
        {onAssociateBranch ? (
          <Button
            type="button"
            variant="secondary"
            className="wt__add-branch"
            onClick={onAssociateBranch}
          >
            Add branch
          </Button>
        ) : null}
      </div>

      <div className="wt__row-actions">
        <Button
          type="button"
          variant="secondary"
          className="wt__delete"
          disabled={deleteDisabled || worktree.is_main}
          title={worktree.is_main ? "Main worktree cannot be deleted" : undefined}
          onClick={onDelete}
        >
          Delete
        </Button>
      </div>
    </article>
  );
}
