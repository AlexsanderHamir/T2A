import type { GitBranch } from "@/types/git";
import { WorktreesBranchIcon } from "./WorktreesIcons";

type Props = {
  branch: GitBranch;
  running?: boolean;
};

export function BranchPill({ branch, running = false }: Props) {
  return (
    <span className="worktrees-branch-control" data-running={running ? "true" : "false"}>
      <WorktreesBranchIcon className="worktrees-branch-control__icon" />
      <span className="worktrees-branch-control__name">{branch.name}</span>
      {running ? (
        <span className="worktrees-branch-control__badge" title="Task running on this branch">
          Running
        </span>
      ) : null}
    </span>
  );
}
