import type { GitBranch, GitWorktree } from "@/types/git";
import {
  worktreeAriaLabel,
  worktreeGitCopy,
} from "../worktreeGitCopy";
import { BranchPill } from "./BranchPill";
import { WorktreesMenu } from "./WorktreesMenu";
import { WorktreesMoreIcon } from "./WorktreesIcons";
import { WorktreesPathChip } from "./WorktreesPathChip";

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
  const branchById = new Map(branches.map((b) => [b.id, b]));
  const branch = worktree.branch_id ? branchById.get(worktree.branch_id) : undefined;
  const deleteBlocked = deleteDisabled || worktree.is_main;
  const kindLabel = worktree.is_main ? worktreeGitCopy.mainWorktreeShortLabel : null;

  const deleteMenuItem = {
    id: "delete-worktree",
    label: worktreeGitCopy.deleteWorktree,
    onSelect: onDelete,
    disabled: deleteBlocked,
    danger: true,
  };

  return (
    <li
      className="draft-row worktree-row"
      data-main={worktree.is_main ? "true" : "false"}
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
        <WorktreesPathChip path={worktree.path} compact />
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

      <div
        className="worktree-row__status"
        title={worktreeGitCopy.statusUnavailableTitle}
        aria-label={worktreeGitCopy.statusUnavailableTitle}
      >
        <span className="worktree-row__status-value" aria-hidden>
          {worktreeGitCopy.statusUnavailable}
        </span>
      </div>

      <div className="draft-row__actions worktree-row__actions">
        <WorktreesMenu
          triggerLabel={worktreeGitCopy.worktreeActions(displayName)}
          className="secondary worktrees-icon-menu-btn"
          icon={<WorktreesMoreIcon />}
          iconOnly
          items={[deleteMenuItem]}
        />
      </div>
    </li>
  );
}
