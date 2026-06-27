import type { GitBranch, GitWorktree } from "@/types/git";
import { BranchPill } from "./BranchPill";

type Props = {
  worktree: GitWorktree;
  branches: GitBranch[];
  onDelete: () => void;
  deleteDisabled?: boolean;
};

export function WorktreeRow({
  worktree,
  branches,
  onDelete,
  deleteDisabled = false,
}: Props) {
  const displayName = worktree.name.trim() || worktree.path;
  const hostHint = worktree.path;
  const branchById = new Map(branches.map((b) => [b.id, b]));
  const branch = worktree.branch_id ? branchById.get(worktree.branch_id) : undefined;

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

      <div className="worktrees-row__branches" aria-label="Branch">
        {branch ? (
          <BranchPill branch={branch} />
        ) : worktree.branch_id ? (
          <span className="worktrees-row__muted">{worktree.branch_id}</span>
        ) : (
          <span className="worktrees-row__muted">No branch</span>
        )}
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
