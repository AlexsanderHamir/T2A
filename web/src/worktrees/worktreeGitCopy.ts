export const worktreeGitCopy = {
  sectionTitle: "Worktrees",
  registerWorktree: "Register worktree",
  createWorktree: "Create worktree",
  reconcile: "Reconcile",
  reconciling: "Reconciling…",
  deleteRepository: "Delete repository",
  hostPathLabel: "Host path",
  defaultBranchLabel: "Default branch",
  listColumnName: "Name",
  listColumnBranch: "Branch",
  listColumnActions: "Actions",
  mainWorktreeLabel: "main worktree",
  mainWorktreeHint:
    "The worktree created by git clone or git init. git worktree remove cannot delete it while linked worktrees exist.",
  detachedHead: "Detached HEAD",
  deleteMainWorktreeTitle:
    "git worktree remove cannot delete the main worktree while linked worktrees exist",
  emptyWorktreesTitle: "No worktrees yet",
  emptyWorktreesDescription:
    "Register an existing linked directory or create a new one with git worktree add.",
  registerModalTitle: "Register worktree",
  registerModalLead:
    "Link an existing git worktree directory and choose the branch Hamix should track.",
  registerModalPathLabel: "Worktree path",
  registerModalPathEmpty:
    "No unregistered linked worktrees found. Use Create worktree or run git worktree add outside Hamix first.",
  registerModalDisplayNameLabel: "Display name",
  registerModalDisplayNamePlaceholder: "Optional",
  registerModalSubmit: "Register worktree",
  registerModalSubmitting: "Registering…",
  createModalTitle: "Create worktree",
  createModalLead:
    "Run git worktree add for a new linked directory and choose the checkout branch Hamix registers with it.",
  createModalPathLabel: "Worktree path",
  createModalChoosePath: "Choose worktree path",
  createModalPathSelectedPrefix: "Path:",
  createModalDisplayNameLabel: "Display name",
  createModalDisplayNamePlaceholder: "Optional",
  createModalSubmit: "Create worktree",
  createModalSubmitting: "Creating…",
  cancel: "Cancel",
} as const;

export function worktreeAriaLabel(displayName: string): string {
  return `Worktree: ${displayName}`;
}

export function deleteWorktreeAriaLabel(displayName: string): string {
  return `Delete worktree "${displayName}"`;
}

export function cannotDeleteMainWorktreeAriaLabel(displayName: string): string {
  return `Cannot delete main worktree "${displayName}"`;
}

export function liveWorktreeOptionLabel(path: string, isMain: boolean): string {
  return isMain ? `${path} (main worktree)` : path;
}
