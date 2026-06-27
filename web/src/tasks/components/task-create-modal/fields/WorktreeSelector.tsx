import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { DEFAULT_PROJECT_ID } from "@/types";
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import { CustomSelect } from "@/components/custom-select";
import { useGlobalRepositories } from "@/worktrees/hooks/useGlobalRepositories";
import { useGlobalWorktrees } from "@/worktrees/hooks/useGlobalWorktrees";
import { useGlobalBranches } from "@/worktrees/hooks/useGlobalBranches";

type Props = {
  idsPrefix: string;
  /** Current project scoping. Non-default project drives Case B (project → repo). */
  projectId?: string;
  worktreeId: string;
  onWorktreeChange: (worktreeId: string) => void;
  disabled?: boolean;
};

export function WorktreeSelector({
  idsPrefix,
  projectId,
  worktreeId,
  onWorktreeChange,
  disabled = false,
}: Props) {
  const projectsEnabled = !isUiFeatureOmitted("projects");
  const hasProject =
    projectsEnabled &&
    typeof projectId === "string" &&
    projectId.trim() !== "" &&
    projectId !== DEFAULT_PROJECT_ID;

  const repositoriesQuery = useGlobalRepositories();
  const repositories = repositoriesQuery.data ?? [];

  const [selectedRepoId, setSelectedRepoId] = useState("");

  useEffect(() => {
    if (repositories.length === 1 && selectedRepoId === "") {
      setSelectedRepoId(repositories[0]!.id);
    }
  }, [repositories, selectedRepoId]);

  const worktreesQuery = useGlobalWorktrees(selectedRepoId, {
    enabled: selectedRepoId !== "",
  });
  const worktrees = worktreesQuery.data ?? [];

  const branchesQuery = useGlobalBranches(selectedRepoId, {
    enabled: selectedRepoId !== "",
  });
  const branches = branchesQuery.data ?? [];
  const branchById = new Map(branches.map((b) => [b.id, b]));

  useEffect(() => {
    if (worktrees.length === 1 && worktreeId === "") {
      onWorktreeChange(worktrees[0]!.id);
    }
  }, [worktrees, worktreeId, onWorktreeChange]);

  const loading =
    repositoriesQuery.isLoading ||
    worktreesQuery.isLoading ||
    branchesQuery.isLoading;

  const noRepositories = !repositoriesQuery.isLoading && repositories.length === 0;

  const repoOptions = repositories.map((r) => ({
    value: r.id,
    label: r.path,
  }));

  const worktreeOptions = worktrees.map((wt) => {
    const branch = wt.branch_id ? branchById.get(wt.branch_id) : undefined;
    const name = wt.name.trim() || wt.path;
    const label = branch ? `${name} (${branch.name})` : name;
    return { value: wt.id, label };
  });

  if (noRepositories) {
    return (
      <p className="worktrees-git-selector__manage">
        No repositories registered.{" "}
        <Link to="/worktrees?register=1" target="_blank" rel="noopener noreferrer">
          Register repository
        </Link>
      </p>
    );
  }

  return (
    <div className="worktrees-git-selector" aria-busy={loading ? "true" : undefined}>
      {hasProject ? (
        <p className="worktrees-git-selector__project-hint">
          Using project repository
        </p>
      ) : (
        <CustomSelect
          id={`${idsPrefix}-repo`}
          label="Repository"
          value={selectedRepoId}
          options={repoOptions}
          disabled={disabled || repositoriesQuery.isLoading || repoOptions.length === 0}
          requirement="required"
          onChange={(id) => {
            setSelectedRepoId(id);
            onWorktreeChange("");
          }}
        />
      )}

      <CustomSelect
        id={`${idsPrefix}-worktree`}
        label="Worktree"
        value={worktreeId}
        options={worktreeOptions}
        disabled={
          disabled ||
          selectedRepoId === "" ||
          worktreesQuery.isLoading ||
          worktreeOptions.length === 0
        }
        requirement="required"
        onChange={onWorktreeChange}
      />

      <p className="worktrees-git-selector__manage">
        <Link to="/worktrees" target="_blank" rel="noopener noreferrer">
          Manage worktrees
        </Link>
      </p>
    </div>
  );
}
