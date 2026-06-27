/** Last path segment for display (e.g. `C:/proj/hamix` → `hamix`). */
export function repositoryDisplayName(path: string): string {
  const normalized = path.trim().replace(/\\/g, "/").replace(/\/+$/, "");
  if (normalized === "") return path;
  const slash = normalized.lastIndexOf("/");
  return slash >= 0 ? normalized.slice(slash + 1) : normalized;
}

function normalizeRepoPath(path: string): string {
  return path.trim().replace(/\\/g, "/").replace(/\/+$/, "").toLowerCase();
}

/** True when two repository paths refer to the same directory. */
export function repositoryPathsEquivalent(a: string, b: string): boolean {
  return normalizeRepoPath(a) === normalizeRepoPath(b);
}

/** Splits a filesystem path into parent directory and final segment for scannable row display. */
export function splitWorktreePath(path: string): { parent: string; base: string } {
  const trimmed = path.trim();
  if (trimmed === "") return { parent: "", base: "" };
  const normalized = trimmed.replace(/\\/g, "/").replace(/\/+$/, "");
  const slash = normalized.lastIndexOf("/");
  if (slash < 0) return { parent: "", base: trimmed };
  const base = normalized.slice(slash + 1);
  const parent = trimmed.slice(0, trimmed.length - base.length);
  return { parent, base };
}

/** Hide worktree path in the row when it duplicates the repository header path. */
export function shouldShowWorktreePath(worktreePath: string, repositoryPath: string): boolean {
  return !repositoryPathsEquivalent(worktreePath, repositoryPath);
}
