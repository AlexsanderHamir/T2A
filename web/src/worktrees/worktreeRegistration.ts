import type { GitWorktree } from "@/types/git";

/** Branch-bound rows only — matches backend gitWorktreeIsFullyRegistered. */
export function isFullyRegisteredWorktree(worktree: GitWorktree): boolean {
  return Boolean(worktree.branch_id?.trim());
}

/** Rows shown on /worktrees: operator-registered linked worktrees (not repo checkout stubs). */
export function isLinkedWorktreeForDisplay(worktree: GitWorktree): boolean {
  return isFullyRegisteredWorktree(worktree) && !worktree.is_main;
}
