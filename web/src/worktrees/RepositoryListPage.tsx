import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import type { GitRepository } from "@/types";
import { Button } from "@/components/ui";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { EmptyState } from "@/shared/EmptyState";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TASK_TIMINGS } from "@/constants/tasks";
import { TaskDraftsListSkeleton } from "@/components/skeletons/TaskDraftsListSkeleton";
import { useGlobalRepositories } from "./hooks/useGlobalRepositories";
import { useGlobalGitMutations } from "./hooks/useGlobalGitMutations";
import { RegisterRepositoryModal } from "./modals/RegisterRepositoryModal";
import { repositoryDisplayName } from "./repositoryDisplay";
import {
  deriveWorktreesPageMode,
  worktreesPageErrorMessage,
  worktreesPageTitle,
} from "./worktreesPageMode";

function RepositoryListRow({
  repository,
  index,
}: {
  repository: GitRepository;
  index: number;
}) {
  const name = repositoryDisplayName(repository.path);
  const to = `/worktrees/${encodeURIComponent(repository.id)}`;

  return (
    <Link
      to={to}
      className="wl__row"
      style={{ animationDelay: `${index * 40}ms` }}
      aria-label={`Open repository ${name}`}
    >
      <div className="wl__row-marker" aria-hidden="true" />
      <div className="wl__row-main">
        <span className="wl__row-name">{name}</span>
        <span className="wl__row-desc" title={repository.path}>
          {repository.path}
        </span>
      </div>
      <svg
        className="wl__row-arrow"
        width="16"
        height="16"
        viewBox="0 0 16 16"
        fill="none"
        aria-hidden="true"
      >
        <path
          d="M6 4l4 4-4 4"
          stroke="currentColor"
          strokeWidth="1.5"
          strokeLinecap="round"
          strokeLinejoin="round"
        />
      </svg>
    </Link>
  );
}

export function RepositoryListPage() {
  const repositoriesQuery = useGlobalRepositories();
  const mutations = useGlobalGitMutations();
  const [searchParams, setSearchParams] = useSearchParams();
  const [registerOpen, setRegisterOpen] = useState(false);

  const repositories = repositoriesQuery.data ?? [];
  const pageMode = deriveWorktreesPageMode({
    isLoading: repositoriesQuery.isLoading && !repositoriesQuery.data,
    isError: repositoriesQuery.isError,
    repositoryCount: repositories.length,
  });
  const pageTitle = worktreesPageTitle();
  useDocumentTitle(pageTitle);

  const showSkeleton = useDelayedTrue(
    pageMode === "loading",
    TASK_TIMINGS.draftResumeMinLoadingMs,
  );

  useEffect(() => {
    if (searchParams.get("register") !== "1") return;
    setRegisterOpen(true);
    setSearchParams({}, { replace: true });
  }, [searchParams, setSearchParams]);

  return (
    <div className="task-detail-content--enter">
      <section
        className="panel task-list-section-panel worktrees-page"
        aria-labelledby="worktrees-heading"
      >
        <header className="task-list-section-head">
          <div className="task-list-section-head__text">
            <h2 id="worktrees-heading" className="task-list-section-title">
              {pageTitle}
            </h2>
            <p className="wl__subtitle">
              Register git checkouts, then open a repository to manage worktrees and branches.
            </p>
          </div>
          <div className="task-list-section-actions">
            {pageMode === "setup" || pageMode === "manage" ? (
              <Button
                type="button"
                variant="primary"
                className="task-home-new-task-btn"
                onClick={() => setRegisterOpen(true)}
              >
                Register repository
              </Button>
            ) : null}
          </div>
        </header>

        {pageMode === "error" ? (
          <div className="err" role="alert">
            <p>{worktreesPageErrorMessage(repositoriesQuery.error)}</p>
            <div className="task-detail-error-actions">
              <button
                type="button"
                className="secondary"
                onClick={() => {
                  void repositoriesQuery.refetch();
                }}
              >
                Try again
              </button>
            </div>
          </div>
        ) : null}

        <div className="task-list-content task-list-content--enter">
          {showSkeleton ? <TaskDraftsListSkeleton /> : null}
          {pageMode === "setup" ? (
            <div className="task-list-empty-cell">
              <EmptyState
                title="Register a repository to get started"
                description="Hamix needs a git checkout before you can register worktrees, bind branches, and run agent tasks."
                hideIcon
                className="empty-state--in-table empty-state--task-list-fresh"
              />
            </div>
          ) : null}
          {pageMode === "manage" ? (
            <div className="wl__list" aria-label="Repositories">
              {repositories.map((repository, index) => (
                <RepositoryListRow
                  key={repository.id}
                  repository={repository}
                  index={index}
                />
              ))}
            </div>
          ) : null}
        </div>
      </section>

      <RegisterRepositoryModal
        open={registerOpen}
        pending={mutations.createRepository.isPending}
        error={mutations.createRepository.error}
        onClose={() => {
          setRegisterOpen(false);
          mutations.createRepository.reset();
        }}
        onSubmit={(input) => {
          void mutations.createRepository
            .mutateAsync(input)
            .then(() => setRegisterOpen(false));
        }}
      />
    </div>
  );
}
