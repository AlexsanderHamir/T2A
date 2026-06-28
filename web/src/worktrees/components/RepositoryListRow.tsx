import type { KeyboardEvent } from "react";
import { useNavigate } from "react-router-dom";
import type { GitRepository } from "@/types/git";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import {
  repositoryDisplayName,
  repositoryPathsEquivalent,
} from "../repositoryDisplay";
import {
  worktreeGitCopy,
  repositoryListWorktreeCountDisplay,
  worktreeCountLabel,
} from "../worktreeGitCopy";
import { isLinkedWorktreeForDisplay } from "../worktreeRegistration";
import { WorktreesChevronRightIcon, WorktreesFolderIcon } from "./WorktreesIcons";

type Props = {
  repository: GitRepository;
};

function isRowActionExcluded(target: EventTarget | null): boolean {
  if (!(target instanceof Element)) return true;
  return Boolean(target.closest("button, a, input, label"));
}

export function RepositoryListRow({ repository }: Props) {
  const navigate = useNavigate();
  const worktreesQuery = useGlobalWorktrees(repository.id);
  const worktrees = (worktreesQuery.data ?? []).filter(isLinkedWorktreeForDisplay);
  const loading = worktreesQuery.isLoading;
  const repoName = repositoryDisplayName(repository.path);
  const showHostPath =
    repository.host_path.trim() !== "" &&
    !repositoryPathsEquivalent(repository.path, repository.host_path);

  const openDetail = () => {
    navigate(`/worktrees/${repository.id}`);
  };

  const onKeyDown = (event: KeyboardEvent<HTMLLIElement>) => {
    if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      openDetail();
    }
  };

  const worktreeCount = worktrees.length;
  const worktreeCountText = worktreeCountLabel(worktreeCount);

  return (
    <li
      className="repository-list-row draft-row draft-row--interactive"
      role="row"
      tabIndex={0}
      aria-label={`${repoName}, ${worktreeCountText}`}
      onClick={(event) => {
        if (isRowActionExcluded(event.target)) return;
        openDetail();
      }}
      onKeyDown={onKeyDown}
    >
      <div className="draft-row__meta" role="gridcell">
        <span className="draft-row__name" title={repoName}>
          {repoName}
        </span>
        <span className="repository-list-row__path draft-row__time" title={repository.path}>
          <WorktreesFolderIcon className="repository-list-row__path-icon" aria-hidden />
          <span className="repository-list-row__path-text">{repository.path}</span>
        </span>
        {showHostPath ? (
          <span className="repository-list-row__host-path draft-row__time">
            <span className="worktrees-repo-row__meta-label">
              {worktreeGitCopy.hostPathLabel}
            </span>
            <code>{repository.host_path}</code>
          </span>
        ) : null}
      </div>
      <span
        className="repository-list-row__count"
        role="gridcell"
        aria-label={worktreeCountText}
      >
        {loading ? (
          <span className="worktrees-inventory-row__count-muted">…</span>
        ) : (
          repositoryListWorktreeCountDisplay(worktreeCount)
        )}
      </span>
      <span className="repository-list-row__chevron" aria-hidden="true">
        <WorktreesChevronRightIcon />
      </span>
    </li>
  );
}
