import type { GitLiveWorktree, GitWorktree } from "@/types/git";
import { worktreeGitCopy } from "./worktreeGitCopy";

export function worktreeStatusLabel(
  live: GitLiveWorktree | undefined,
  worktree: GitWorktree,
): { label: string; title: string } {
  if (live?.locked) {
    return {
      label: worktreeGitCopy.statusLocked,
      title: worktreeGitCopy.statusLockedTitle,
    };
  }
  if (live?.prunable) {
    return {
      label: worktreeGitCopy.statusPrunable,
      title: worktreeGitCopy.statusPrunableTitle,
    };
  }
  if (live?.detached) {
    return {
      label: worktreeGitCopy.detachedHead,
      title: worktreeGitCopy.detachedHeadTitle,
    };
  }
  if (!worktree.branch_id) {
    return {
      label: worktreeGitCopy.needsBranchBind,
      title: worktreeGitCopy.needsBranchBindTitle,
    };
  }
  if (live) {
    return {
      label: worktreeGitCopy.statusReady,
      title: worktreeGitCopy.statusReadyTitle,
    };
  }
  return {
    label: worktreeGitCopy.statusUnavailable,
    title: worktreeGitCopy.statusUnavailableTitle,
  };
}
