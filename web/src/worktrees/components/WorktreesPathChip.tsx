import { WorktreesFolderIcon } from "./WorktreesIcons";

type Props = {
  path: string;
  compact?: boolean;
};

export function WorktreesPathChip({ path, compact = false }: Props) {
  return (
    <span
      className={`worktrees-path-chip${compact ? " worktrees-path-chip--compact" : ""}`}
      title={path}
    >
      <WorktreesFolderIcon className="worktrees-path-chip__icon" />
      <span className="worktrees-path-chip__text">{path}</span>
    </span>
  );
}
