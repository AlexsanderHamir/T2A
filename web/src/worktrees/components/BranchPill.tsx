import type { GitBranch } from "@/types/git";

type Props = {
  branch: GitBranch;
  running?: boolean;
};

export function BranchPill({ branch, running = false }: Props) {
  return (
    <span className="worktrees-branch-pill" data-running={running ? "true" : "false"}>
      <span className="worktrees-branch-pill__name">{branch.name}</span>
      {running ? (
        <span className="worktrees-branch-pill__badge" title="Task running on this branch">
          Running
        </span>
      ) : null}
    </span>
  );
}
