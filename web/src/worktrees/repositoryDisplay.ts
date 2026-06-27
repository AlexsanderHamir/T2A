/** Last path segment for display (e.g. `C:/proj/T2A` → `T2A`). */
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
