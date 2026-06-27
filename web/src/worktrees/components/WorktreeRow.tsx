import type { GitBranch, GitWorktree } from "@/types/git";
import { TaskListDeleteGlyph } from "@/shared/ListRowActionGlyphs";
import {
  shouldShowWorktreePath,
  splitWorktreePath,
} from "../repositoryDisplay";
import {
  cannotDeleteMainWorktreeAriaLabel,
  deleteWorktreeAriaLabel,
  worktreeAriaLabel,
  worktreeGitCopy,
} from "../worktreeGitCopy";
import { BranchPill } from "./BranchPill";

type Props = {
  worktree: GitWorktree;
  branches: GitBranch[];
  repositoryPath: string;
  onDelete: () => void;
  deleteDisabled?: boolean;
};

export function WorktreeRow({
  worktree,
  branches,
  repositoryPath,
  onDelete,
  deleteDisabled = false,
}: Props) {
  const displayName = worktree.name.trim() || worktree.path;
  const hostHint = worktree.path;
  const branchById = new Map(branches.map((b) => [b.id, b]));
  const branch = worktree.branch_id ? branchById.get(worktree.branch_id) : undefined;
  const deleteBlocked = deleteDisabled || worktree.is_main;
  const kindLabel = worktree.is_main ? worktreeGitCopy.mainWorktreeLabel : null;
  const showPath = shouldShowWorktreePath(worktree.path, repositoryPath);
  const pathParts = splitWorktreePath(hostHint);
  const showPathBase = pathParts.base !== displayName;
  const showPathLine = showPath && (pathParts.parent !== "" || showPathBase);

  return (
    <li
      className="draft-row worktree-row"
      data-main={worktree.is_main ? "true" : "false"}
      data-has-path={showPathLine ? "true" : "false"}
      aria-label={worktreeAriaLabel(displayName)}
    >
      <div className="draft-row__meta worktree-row__meta">
        <div className="worktree-row__title">
          <span className="draft-row__name" title={displayName}>
            {displayName}
          </span>
          {kindLabel ? (
            <span
              className="worktree-row__kind"
              title={worktreeGitCopy.mainWorktreeHint}
            >
              {kindLabel}
            </span>
          ) : null}
        </div>
        {showPathLine ? (
          <span className="worktree-row__path-line" title={hostHint}>
            {pathParts.parent ? (
              <span className="worktree-row__path-parent">{pathParts.parent}</span>
            ) : null}
            {showPathBase ? (
              <span className="worktree-row__path-base">{pathParts.base}</span>
            ) : null}
          </span>
        ) : null}
      </div>

      <div className="worktree-row__branch" aria-label="Branch">
        {branch ? (
          <BranchPill branch={branch} />
        ) : worktree.branch_id ? (
          <span className="worktree-row__branch-empty">{worktree.branch_id}</span>
        ) : (
          <span className="worktree-row__branch-empty">{worktreeGitCopy.detachedHead}</span>
        )}
      </div>

      <div className="draft-row__actions worktree-row__actions">
        <div className="task-list-row-actions">
          <button
            type="button"
            className="task-list-icon-btn task-list-icon-btn--delete"
            disabled={deleteBlocked}
            title={
              worktree.is_main ? worktreeGitCopy.deleteMainWorktreeTitle : undefined
            }
            aria-label={
              worktree.is_main
                ? cannotDeleteMainWorktreeAriaLabel(displayName)
                : deleteWorktreeAriaLabel(displayName)
            }
            onClick={(e) => {
              e.stopPropagation();
              onDelete();
            }}
          >
            <TaskListDeleteGlyph />
          </button>
        </div>
      </div>
    </li>
  );
}
