const COPY = {
  loading: "Loading linked worktrees…",
  empty: "No unregistered linked worktrees for this repository.",
  prompt: "Select a linked worktree",
} as const;

export type RegisterWorktreePathSelectState = {
  loading: boolean;
  optionCount: number;
};

export function registerWorktreePathPlaceholder(
  state: RegisterWorktreePathSelectState,
): string {
  if (state.loading) return COPY.loading;
  if (state.optionCount === 0) return COPY.empty;
  return COPY.prompt;
}

export function registerWorktreePathDisabled(
  state: RegisterWorktreePathSelectState & { pending: boolean },
): boolean {
  return state.pending || state.loading || state.optionCount === 0;
}
