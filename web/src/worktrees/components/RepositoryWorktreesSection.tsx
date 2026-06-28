import type { GitRepository } from "@/types/git";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { useGlobalBranches } from "../hooks/useGlobalBranches";
import { useGlobalWorktrees } from "../hooks/useGlobalWorktrees";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { isLinkedWorktreeForDisplay } from "../worktreeRegistration";
import { gitReconcileErrorMessage } from "../gitReconcileErrors";
import { WorktreesPlusIcon } from "./WorktreesIcons";
import { WorktreesMenu } from "./WorktreesMenu";
import { WorktreeRow } from "./WorktreeRow";
import { WorktreeReconcileStatus } from "./WorktreeReconcileStatus";

type Props = {
  repository: GitRepository;
  onRegisterWorktree: () => void;
  onCreateWorktree: () => void;
  onDeleteWorktree: (worktreeId: string, label: string) => void;
  reconcilePending?: boolean;
  reconcileError?: unknown;
};

export function RepositoryWorktreesSection({
  repository,
  onRegisterWorktree,
  onCreateWorktree,
  onDeleteWorktree,
  reconcilePending = false,
  reconcileError,
}: Props) {
  const worktreesQuery = useGlobalWorktrees(repository.id);
  const branchesQuery = useGlobalBranches(repository.id);
  const worktrees = (worktreesQuery.data ?? []).filter(isLinkedWorktreeForDisplay);
  const branches = branchesQuery.data ?? [];
  const loading = worktreesQuery.isLoading || branchesQuery.isLoading;
  const worktreesError =
    worktreesQuery.isError && !worktreesQuery.isLoading
      ? worktreesQuery.error instanceof Error
        ? worktreesQuery.error.message
        : "Could not load worktrees."
      : null;
  const reconcileErrorMessage =
    reconcileError != null ? gitReconcileErrorMessage(reconcileError) : null;

  return (
    <div className="worktrees-inventory worktrees-inventory--detail">
      <div
        className="worktrees-inventory-head worktrees-inventory-head--detail"
        role="row"
      >
        <span
          className="worktrees-inventory-head__label worktrees-inventory-head__label--name"
          role="columnheader"
        >
          {worktreeGitCopy.listColumnName}
        </span>
        <span
          className="worktrees-inventory-head__label worktrees-inventory-head__label--branch"
          role="columnheader"
        >
          {worktreeGitCopy.listColumnBranch}
        </span>
        <span
          className="worktrees-inventory-head__label worktrees-inventory-head__label--menu"
          aria-hidden
        />
      </div>
      <ul className="worktrees-inventory-rows" aria-label="Worktrees">
        {reconcilePending ? (
          <li className="worktrees-inventory-row worktrees-inventory-row--status">
            <WorktreeReconcileStatus className="worktrees-inventory-reconcile" />
          </li>
        ) : null}

        {reconcileErrorMessage ? (
          <li className="worktrees-inventory-row worktrees-inventory-row--status">
            <MutationErrorBanner
              error={reconcileErrorMessage}
              className="worktrees-inventory-error"
            />
          </li>
        ) : null}

        {worktreesError ? (
          <li className="worktrees-inventory-row worktrees-inventory-row--status">
            <MutationErrorBanner error={worktreesError} className="worktrees-inventory-error" />
          </li>
        ) : null}

        {loading ? (
          <li className="worktrees-inventory-row worktrees-inventory-row--status">
            <p className="worktrees-inventory-loading" aria-busy="true">
              Loading worktrees…
            </p>
          </li>
        ) : null}

        {!loading && !worktreesError
          ? worktrees.map((worktree) => (
              <WorktreeRow
                key={worktree.id}
                worktree={worktree}
                branches={branches}
                onDelete={() =>
                  onDeleteWorktree(worktree.id, worktree.name.trim() || worktree.path)
                }
              />
            ))
          : null}

        {!loading && !worktreesError && worktrees.length === 0 ? (
          <li className="worktrees-inventory-row worktrees-inventory-row--empty">
            <p className="worktrees-inventory-empty">{worktreeGitCopy.emptyWorktreesTitle}</p>
          </li>
        ) : null}

        <li className="worktrees-inventory-row worktrees-inventory-row--add">
          <div className="worktrees-inventory-row__name">
            <WorktreesMenu
              triggerLabel={worktreeGitCopy.addWorktree}
              className="worktrees-inventory-add-btn"
              icon={
                <WorktreesPlusIcon className="worktrees-inventory-add-btn__icon" aria-hidden />
              }
              items={[
                {
                  id: "register-worktree",
                  label: worktreeGitCopy.registerWorktree,
                  onSelect: onRegisterWorktree,
                },
                {
                  id: "create-worktree",
                  label: worktreeGitCopy.createWorktree,
                  onSelect: onCreateWorktree,
                },
              ]}
            />
          </div>
        </li>
      </ul>
    </div>
  );
}
