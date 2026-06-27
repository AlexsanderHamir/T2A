/** Parent directory of a filesystem path for relocate folder browsing. */
export function parentBrowsePath(path: string): string {
  const trimmed = path.trim().replace(/[\\/]+$/, "");
  if (trimmed === "") return "";
  const idx = Math.max(trimmed.lastIndexOf("/"), trimmed.lastIndexOf("\\"));
  if (idx <= 0) return "";
  return trimmed.slice(0, idx);
}
