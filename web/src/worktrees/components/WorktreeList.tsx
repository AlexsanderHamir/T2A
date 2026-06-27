import type { GitBranch, GitWorktree } from "@/types/git";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { WorktreeRow } from "./WorktreeRow";

type Props = {
  worktrees: GitWorktree[];
  branches: GitBranch[];
  onDeleteWorktree: (worktreeId: string, label: string) => void;
};

export function WorktreeList({ worktrees, branches, onDeleteWorktree }: Props) {
  return (
    <div className="worktree-list table-wrap">
      <div className="worktree-list-head" role="row">
        <span className="worktree-list-head__label" role="columnheader">
          {worktreeGitCopy.listColumnName}
        </span>
        <span
          className="worktree-list-head__label worktree-list-head__label--branch"
          role="columnheader"
        >
          {worktreeGitCopy.listColumnBranch}
        </span>
        <span
          className="worktree-list-head__label worktree-list-head__label--actions"
          role="columnheader"
        >
          {worktreeGitCopy.listColumnActions}
        </span>
      </div>
      <ul className="draft-row-list worktree-list-rows" aria-label="Worktrees">
        {worktrees.map((worktree) => (
          <WorktreeRow
            key={worktree.id}
            worktree={worktree}
            branches={branches}
            onDelete={() =>
              onDeleteWorktree(worktree.id, worktree.name.trim() || worktree.path)
            }
          />
        ))}
      </ul>
    </div>
  );
}
