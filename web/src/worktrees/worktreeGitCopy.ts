export const worktreeGitCopy = {
  sectionTitle: "Worktrees",
  addWorktree: "Add worktree",
  registerRepository: "Register repository",
  registerWorktree: "Register worktree",
  createWorktree: "Create worktree",
  reconcile: "Reconcile",
  reconciling: "Reconciling…",
  deleteRepository: "Delete repository",
  deleteWorktree: "Delete worktree",
  repositoryActions: "Repository actions",
  worktreeActions: (name: string) => `Worktree actions for ${name}`,
  hostPathLabel: "Host path",
  listColumnName: "Name",
  listColumnBranch: "Branch",
  listColumnStatus: "Status",
  mainWorktreeShortLabel: "main",
  mainWorktreeLabel: "main worktree",
  mainWorktreeHint:
    "The primary checkout from git clone or git init. Deleting removes Hamix registration only — the checkout stays on disk.",
  statusUnavailable: "—",
  statusUnavailableTitle: "Worktree checkout status is not available yet",
  detachedHead: "Detached HEAD",
  emptyWorktreesTitle: "No worktrees yet",
  emptyWorktreesDescription:
    "Register an existing linked directory or create a new one with git worktree add.",
  registerModalTitle: "Register worktree",
  registerModalLead:
    "Link an existing git worktree directory and choose the branch Hamix should track.",
  registerModalPathLabel: "Worktree path",
  registerModalDisplayNameLabel: "Display name",
  registerModalDisplayNamePlaceholder: "Optional",
  registerModalSubmit: "Register worktree",
  registerModalSubmitting: "Registering…",
  createModalTitle: "Create worktree",
  createModalLead:
    "Run git worktree add from the main checkout. Choose whether new branches start from main or from an existing linked worktree.",
  createModalStartFromLabel: "Start from",
  createModalStartFromMain: "Main repository checkout",
  createModalStartFromReference: "Reference worktree",
  createModalReferenceLabel: "Reference worktree",
  createModalReferenceDetached:
    "The selected worktree has a detached HEAD. Pick a worktree checked out on a branch.",
  createModalPathLabel: "Worktree path",
  createModalChoosePath: "Choose worktree path",
  createModalPathSelectedPrefix: "Path:",
  createModalDisplayNameLabel: "Display name",
  createModalDisplayNamePlaceholder: "Optional",
  createModalSubmit: "Create worktree",
  createModalSubmitting: "Creating…",
  cancel: "Cancel",
  relocateModalTitle: "Relocate repository",
  relocateModalLead:
    "Hamix cannot find this repository at its registered path. Choose the checkout on disk — Hamix verifies it is the same repo before updating paths.",
  relocateModalStoredPathLabel: "Registered path",
  relocateModalPathLabel: "New checkout path",
  relocateModalChoosePath: "Choose folder",
  relocateModalSelectedPrefix: "Selected:",
  relocateModalNoPath: "No folder selected yet.",
  relocateModalSubmit: "Relocate and reconcile",
  relocateModalSubmitting: "Relocating…",
  reconcileErrorTitle: "Reconcile failed",
} as const;

export function worktreeAriaLabel(displayName: string): string {
  return `Worktree: ${displayName}`;
}

export function deleteWorktreeAriaLabel(displayName: string): string {
  return `Delete worktree "${displayName}"`;
}

export function liveWorktreeOptionLabel(path: string, isMain: boolean): string {
  return isMain ? `${path} (main worktree)` : path;
}
