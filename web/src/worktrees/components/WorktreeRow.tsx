import type { GitBranch, GitWorktree } from "@/types/git";
import { useWorktreeBranchAssociations } from "../hooks/useWorktreeBranchAssociations";
import { BranchPill } from "./BranchPill";

type Props = {
  worktree: GitWorktree;
  branches: GitBranch[];
  onDelete: () => void;
  deleteDisabled?: boolean;
  onAssociateBranch?: () => void;
  onDeleteAssociation?: (assocId: string, branchId: string, label: string) => void;
};

export function WorktreeRow({
  worktree,
  branches,
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
    <div className="worktrees-row" data-main={worktree.is_main ? "true" : "false"}>
      <div className="worktrees-row__main">
        <p className="worktrees-row__name">{displayName}</p>
        <p className="worktrees-row__path" title={hostHint}>
          <code>{hostHint}</code>
        </p>
        {worktree.is_main ? (
          <span className="worktrees-row__badge">Main worktree</span>
        ) : null}
      </div>

      <div className="worktrees-row__branches" aria-label="Associated branches">
        {associationsQuery.isLoading ? (
          <span className="worktrees-row__muted">Loading…</span>
        ) : associations.length === 0 ? (
          <span className="worktrees-row__muted">No branches</span>
        ) : (
          associations.map((assoc) => {
            const branch = branchById.get(assoc.branch_id);
            const label = branch?.name ?? assoc.branch_id;
            return (
              <span key={assoc.id} className="worktrees-row__assoc">
                {branch ? <BranchPill branch={branch} /> : (
                  <span className="worktrees-row__muted">{label}</span>
                )}
                {onDeleteAssociation ? (
                  <button
                    type="button"
                    className="secondary worktrees-row__delete-assoc"
                    onClick={() => onDeleteAssociation(assoc.id, assoc.branch_id, label)}
                  >
                    Delete
                  </button>
                ) : null}
              </span>
            );
          })
        )}
        {onAssociateBranch ? (
          <button
            type="button"
            className="secondary worktrees-row__add-branch"
            onClick={onAssociateBranch}
          >
            Associate branch
          </button>
        ) : null}
      </div>

      <div className="worktrees-row__actions">
        <button
          type="button"
          className="secondary worktrees-row__delete"
          disabled={deleteDisabled || worktree.is_main}
          title={worktree.is_main ? "Main worktree cannot be deleted" : undefined}
          onClick={onDelete}
        >
          Delete
        </button>
      </div>
    </div>
  );
}
