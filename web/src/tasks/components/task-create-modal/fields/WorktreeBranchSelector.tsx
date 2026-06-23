import { useEffect } from "react";
import { Link } from "react-router-dom";
import { DEFAULT_PROJECT_ID } from "@/types";
import { CustomSelect } from "@/tasks/components/custom-select/CustomSelect";
import { FieldLabel } from "@/shared/FieldLabel";
import { useRepositories } from "@/worktrees/hooks/useRepositories";
import { useWorktrees } from "@/worktrees/hooks/useWorktrees";
import { useBranches } from "@/worktrees/hooks/useBranches";

type Props = {
  idsPrefix: string;
  projectId: string;
  worktreeId: string;
  branchId: string;
  disabled?: boolean;
  onWorktreeChange: (worktreeId: string) => void;
  onBranchChange: (branchId: string) => void;
};

export function WorktreeBranchSelector({
  idsPrefix,
  projectId,
  worktreeId,
  branchId,
  disabled = false,
  onWorktreeChange,
  onBranchChange,
}: Props) {
  const effectiveProjectId = projectId.trim() || DEFAULT_PROJECT_ID;
  const repositoriesQuery = useRepositories(effectiveProjectId);
  const repository = repositoriesQuery.data?.[0];
  const repositoryId = repository?.id ?? "";

  const worktreesQuery = useWorktrees(effectiveProjectId, repositoryId, {
    enabled: repositoryId !== "",
  });
  const branchesQuery = useBranches(effectiveProjectId, repositoryId, {
    enabled: repositoryId !== "",
  });

  const worktrees = worktreesQuery.data ?? [];
  const branches = branchesQuery.data ?? [];
  const loading =
    repositoriesQuery.isLoading || worktreesQuery.isLoading || branchesQuery.isLoading;

  useEffect(() => {
    if (worktrees.length === 1 && worktreeId === "") {
      onWorktreeChange(worktrees[0]!.id);
    }
  }, [worktrees, worktreeId, onWorktreeChange]);

  useEffect(() => {
    if (branches.length === 1 && branchId === "") {
      onBranchChange(branches[0]!.id);
    }
  }, [branches, branchId, onBranchChange]);

  const worktreeOptions = worktrees.map((wt) => ({
    value: wt.id,
    label: wt.name.trim() || wt.path,
  }));

  const branchOptions = branches.map((b) => ({
    value: b.id,
    label: b.name,
  }));

  const noRepositories = !loading && (repositoriesQuery.data?.length ?? 0) === 0;

  return (
    <div className="worktrees-git-selector" aria-busy={loading ? "true" : undefined}>
      {noRepositories ? (
        <p className="worktrees-git-selector__manage">
          No repositories registered.{" "}
          <Link to="/worktrees" target="_blank" rel="noopener noreferrer">
            Manage worktrees
          </Link>
        </p>
      ) : (
        <>
          <div className="field">
            <FieldLabel htmlFor={`${idsPrefix}-worktree`} requirement="required">
              Worktree
            </FieldLabel>
            <CustomSelect
              id={`${idsPrefix}-worktree`}
              label="Worktree"
              value={worktreeId}
              options={worktreeOptions}
              disabled={disabled || loading || worktreeOptions.length === 0}
              requirement="required"
              onChange={onWorktreeChange}
            />
          </div>
          <div className="field">
            <FieldLabel htmlFor={`${idsPrefix}-branch`} requirement="required">
              Branch
            </FieldLabel>
            <CustomSelect
              id={`${idsPrefix}-branch`}
              label="Branch"
              value={branchId}
              options={branchOptions}
              disabled={disabled || loading || branchOptions.length === 0}
              requirement="required"
              onChange={onBranchChange}
            />
          </div>
          <p className="worktrees-git-selector__manage">
            <Link to="/worktrees" target="_blank" rel="noopener noreferrer">
              Manage worktrees
            </Link>
          </p>
        </>
      )}
    </div>
  );
}
