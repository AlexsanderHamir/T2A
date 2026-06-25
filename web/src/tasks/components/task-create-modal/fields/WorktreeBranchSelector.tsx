import { useEffect, useState } from "react";
import { Link } from "react-router-dom";
import { DEFAULT_PROJECT_ID } from "@/types";
import { isUiFeatureOmitted } from "@/launch/omittedFeatures";
import { CustomSelect } from "@/tasks/components/custom-select/CustomSelect";
import { useGlobalRepositories } from "@/worktrees/hooks/useGlobalRepositories";
import { useGlobalWorktrees } from "@/worktrees/hooks/useGlobalWorktrees";
import { useGlobalBranches } from "@/worktrees/hooks/useGlobalBranches";
import { useWorktreeBranchAssociations } from "@/worktrees/hooks/useWorktreeBranchAssociations";
import type { WorktreeBranch } from "@/types/git";

type Props = {
  idsPrefix: string;
  /** Current project scoping. Non-default project drives Case B (project → repo). */
  projectId?: string;
  /** Called when the user picks an association (worktree_branch row id). */
  worktreeBranchId: string;
  onWorktreeBranchChange: (id: string) => void;
  /** Optional backward-compat callbacks – fired from the selected association's fields. */
  onWorktreeChange?: (worktreeId: string) => void;
  onBranchChange?: (branchId: string) => void;
  disabled?: boolean;
};

export function WorktreeBranchSelector({
  idsPrefix,
  projectId,
  worktreeBranchId,
  onWorktreeBranchChange,
  onWorktreeChange,
  onBranchChange,
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
  const [selectedWorktreeId, setSelectedWorktreeId] = useState("");

  // Auto-select repo when only one is available.
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

  // Auto-select worktree when only one is available.
  useEffect(() => {
    if (worktrees.length === 1 && selectedWorktreeId === "") {
      setSelectedWorktreeId(worktrees[0]!.id);
    }
  }, [worktrees, selectedWorktreeId]);

  const associationsQuery = useWorktreeBranchAssociations(selectedWorktreeId, {
    enabled: selectedWorktreeId !== "",
  });
  const associations = associationsQuery.data ?? [];

  // Auto-select the only association when it's the sole option.
  useEffect(() => {
    if (associations.length === 1 && worktreeBranchId === "") {
      const assoc = associations[0]!;
      fireAssociation(assoc);
    }
  // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [associations, worktreeBranchId]);

  const branchById = new Map(branches.map((b) => [b.id, b]));

  function fireAssociation(assoc: WorktreeBranch) {
    onWorktreeBranchChange(assoc.id);
    onWorktreeChange?.(assoc.worktree_id);
    onBranchChange?.(assoc.branch_id);
  }

  const loading =
    repositoriesQuery.isLoading ||
    worktreesQuery.isLoading ||
    branchesQuery.isLoading ||
    associationsQuery.isLoading;

  const noRepositories = !repositoriesQuery.isLoading && repositories.length === 0;

  const repoOptions = repositories.map((r) => ({
    value: r.id,
    label: r.path,
  }));

  const worktreeOptions = worktrees.map((wt) => ({
    value: wt.id,
    label: wt.name.trim() || wt.path,
  }));

  const assocOptions = associations.map((assoc) => {
    const branch = branchById.get(assoc.branch_id);
    return {
      value: assoc.id,
      label: branch?.name ?? assoc.branch_id,
    };
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
            setSelectedWorktreeId("");
            onWorktreeBranchChange("");
            onWorktreeChange?.("");
            onBranchChange?.("");
          }}
        />
      )}

      <CustomSelect
        id={`${idsPrefix}-worktree`}
        label="Worktree"
        value={selectedWorktreeId}
        options={worktreeOptions}
        disabled={
          disabled ||
          selectedRepoId === "" ||
          worktreesQuery.isLoading ||
          worktreeOptions.length === 0
        }
        requirement="required"
        onChange={(id) => {
          setSelectedWorktreeId(id);
          onWorktreeBranchChange("");
          onWorktreeChange?.("");
          onBranchChange?.("");
        }}
      />

      <CustomSelect
        id={`${idsPrefix}-association`}
        label="Branch"
        value={worktreeBranchId}
        options={assocOptions}
        disabled={
          disabled ||
          selectedWorktreeId === "" ||
          associationsQuery.isLoading ||
          assocOptions.length === 0
        }
        requirement="required"
        onChange={(id) => {
          const assoc = associations.find((a) => a.id === id);
          if (!assoc) return;
          fireAssociation(assoc);
        }}
      />

      <p className="worktrees-git-selector__manage">
        <Link to="/worktrees" target="_blank" rel="noopener noreferrer">
          Manage worktrees
        </Link>
      </p>
    </div>
  );
}
