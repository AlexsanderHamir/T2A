import { useEffect, useMemo, useState } from "react";
import { useSearchParams } from "react-router-dom";
import { Button } from "@/components/ui";
import { useDocumentTitle } from "@/shared/useDocumentTitle";
import { EmptyState } from "@/shared/EmptyState";
import { useDelayedTrue } from "@/lib/useDelayedTrue";
import { TASK_TIMINGS } from "@/constants/tasks";
import { TaskDraftsListSkeleton } from "@/components/skeletons/TaskDraftsListSkeleton";
import { useGlobalRepositories } from "./hooks/useGlobalRepositories";
import { useGlobalGitMutations } from "./hooks/useGlobalGitMutations";
import { RepositoriesListTable } from "./components/RepositoriesListTable";
import { RegisterRepositoryModal } from "./modals/RegisterRepositoryModal";
import {
  deriveWorktreesPageMode,
  worktreesPageErrorMessage,
  worktreesPageTitle,
} from "./worktreesPageMode";
import { repositoryMatchesSearchQuery } from "./repositoryDisplay";
import { worktreeGitCopy } from "./worktreeGitCopy";
import { WorktreesPlusIcon } from "./components/WorktreesIcons";

function useDebouncedTrimmedValue(value: string, delayMs: number): string {
  const [debounced, setDebounced] = useState(value.trim());

  useEffect(() => {
    const timer = window.setTimeout(() => setDebounced(value.trim()), delayMs);
    return () => window.clearTimeout(timer);
  }, [value, delayMs]);

  return debounced;
}

export function RepositoriesListPage() {
  const repositoriesQuery = useGlobalRepositories();
  const mutations = useGlobalGitMutations();
  const [searchParams, setSearchParams] = useSearchParams();
  const [registerOpen, setRegisterOpen] = useState(false);
  const [searchInput, setSearchInput] = useState("");

  const repositories = repositoriesQuery.data ?? [];
  const debouncedQ = useDebouncedTrimmedValue(searchInput, 300);
  const filteredRepositories = useMemo(
    () => repositories.filter((repo) => repositoryMatchesSearchQuery(repo, debouncedQ)),
    [repositories, debouncedQ],
  );
  const pageMode = deriveWorktreesPageMode({
    isLoading: repositoriesQuery.isLoading && !repositoriesQuery.data,
    isError: repositoriesQuery.isError,
    repositoryCount: repositories.length,
  });
  const pageTitle = worktreesPageTitle();
  useDocumentTitle(pageTitle);
  const showSearch = pageMode === "setup" || pageMode === "manage";

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
    <section
      className="panel task-list-section-panel task-detail-content--enter worktrees-page"
      aria-labelledby="worktrees-heading"
    >
      <div className="task-list-toolbar">
        <header className="task-list-section-head">
          <div className="task-list-section-head__text">
            <h2 id="worktrees-heading" className="task-list-section-title">
              {pageTitle}
            </h2>
          </div>
          <div className="task-list-section-actions">
            {pageMode === "setup" || pageMode === "manage" ? (
              <Button
                type="button"
                variant="primary"
                className="task-home-new-task-btn worktrees-register-btn"
                onClick={() => setRegisterOpen(true)}
              >
                <WorktreesPlusIcon className="worktrees-register-btn__icon" aria-hidden />
                {worktreeGitCopy.registerRepository}
              </Button>
            ) : null}
          </div>
        </header>

        {showSearch ? (
          <div
            className="task-templates-search field grow task-list-search-field"
            role="search"
            aria-label="Search repositories"
          >
            <label htmlFor="repositories-search" className="visually-hidden">
              Search repositories
            </label>
            <input
              id="repositories-search"
              type="search"
              placeholder={worktreeGitCopy.searchRepositoriesPlaceholder}
              autoComplete="off"
              value={searchInput}
              onChange={(e) => setSearchInput(e.target.value)}
            />
          </div>
        ) : null}
      </div>

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

      <div className="stack">
        {showSkeleton ? <TaskDraftsListSkeleton /> : null}
        {!showSkeleton ? (
          <div className="stack task-list-content task-list-content--enter">
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
            {pageMode === "manage" && filteredRepositories.length === 0 && debouncedQ ? (
              <EmptyState
                title="No matching repositories"
                description="Try a different search term."
                hideIcon
                className="empty-state--task-list-fresh"
              />
            ) : null}
            {pageMode === "manage" && filteredRepositories.length > 0 ? (
              <RepositoriesListTable repositories={filteredRepositories} />
            ) : null}
          </div>
        ) : null}
      </div>

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
    </section>
  );
}

/** @deprecated Use RepositoriesListPage — kept for lazy import compatibility during migration. */
export const WorktreesPage = RepositoriesListPage;
