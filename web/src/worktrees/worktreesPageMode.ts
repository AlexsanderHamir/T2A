/** Pure mode derivation for `/worktrees` from global repository query state. */
export type WorktreesPageMode = "loading" | "error" | "setup" | "manage";

export function deriveWorktreesPageMode(input: {
  isLoading: boolean;
  isError: boolean;
  repositoryCount: number;
}): WorktreesPageMode {
  if (input.isLoading) return "loading";
  if (input.isError) return "error";
  if (input.repositoryCount === 0) return "setup";
  return "manage";
}

export function worktreesPageErrorMessage(error: unknown): string {
  const message =
    error instanceof Error ? error.message.trim() : "Could not load repositories.";
  if (message === "Not Found") {
    return "Could not load repositories. The git API may be unavailable — this is different from having zero repositories registered.";
  }
  return message || "Could not load repositories.";
}

export function worktreesPageTitle(): string {
  return "Repositories";
}
