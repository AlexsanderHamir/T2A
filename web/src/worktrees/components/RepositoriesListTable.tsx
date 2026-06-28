import type { GitRepository } from "@/types/git";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { RepositoryListRow } from "./RepositoryListRow";

type Props = {
  repositories: GitRepository[];
};

export function RepositoriesListTable({ repositories }: Props) {
  return (
    <div className="repositories-list">
      <div className="repositories-list-head" role="row">
        <span className="repositories-list-head__label" role="columnheader">
          {worktreeGitCopy.listColumnName}
        </span>
        <span
          className="repositories-list-head__label repositories-list-head__label--count"
          role="columnheader"
        >
          {worktreeGitCopy.listColumnWorktreeCount}
        </span>
        <span className="repositories-list-head__label repositories-list-head__spacer" aria-hidden />
      </div>
      <ul className="draft-row-list repositories-list-rows" aria-label="Repositories">
        {repositories.map((repository) => (
          <RepositoryListRow key={repository.id} repository={repository} />
        ))}
      </ul>
    </div>
  );
}
