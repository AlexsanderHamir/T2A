import { WorktreesFolderIcon } from "./WorktreesIcons";

type Props = {
  path: string;
  /** Defaults to `path`; use for full filesystem path on hover when `path` is shortened. */
  title?: string;
  compact?: boolean;
};

export function WorktreesPathChip({ path, title, compact = false }: Props) {
  return (
    <span
      className={`worktrees-path-chip${compact ? " worktrees-path-chip--compact" : ""}`}
      title={title ?? path}
    >
      <WorktreesFolderIcon className="worktrees-path-chip__icon" />
      <span className="worktrees-path-chip__text">{path}</span>
    </span>
  );
}
