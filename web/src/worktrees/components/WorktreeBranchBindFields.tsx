import { CustomSelect } from "@/components/custom-select";
import { useGlobalLiveBranches } from "../hooks/useGlobalBranches";

type BranchBindValue = {
  selectedBranchName: string;
  newBranchName: string;
  createNew: boolean;
};

type Props = {
  repositoryId: string;
  enabled: boolean;
  pending: boolean;
  value: BranchBindValue;
  onChange: (next: BranchBindValue) => void;
  branchSelectId: string;
  newBranchInputId?: string;
  /** Label for the existing-branch picker (when not creating new). */
  existingBranchLabel?: string;
  /** Helper copy below the existing-branch picker. */
  existingBranchHint?: string;
};

export function WorktreeBranchBindFields({
  repositoryId,
  enabled,
  pending,
  value,
  onChange,
  branchSelectId,
  newBranchInputId = "worktree-branch-new-name",
  existingBranchLabel = "Existing repository branch",
  existingBranchHint = "Branches already registered in git for this repository.",
}: Props) {
  const liveBranchesQuery = useGlobalLiveBranches(repositoryId, { enabled });
  const liveBranches = liveBranchesQuery.data ?? [];
  const branchOptions = liveBranches.map((b) => ({ value: b.name, label: b.name }));

  return (
    <div className="worktrees-form-modal__branch-fields">
      <label className="worktrees-form-modal__checkbox">
        <input
          type="checkbox"
          checked={value.createNew}
          disabled={pending}
          onChange={(e) => onChange({ ...value, createNew: e.target.checked })}
        />
        Create a new branch
      </label>

      {value.createNew ? (
        <label className="field">
          <span className="settings-field-label">New branch name</span>
          <input
            id={newBranchInputId}
            type="text"
            value={value.newBranchName}
            required
            disabled={pending}
            placeholder="e.g. feature-auth"
            onChange={(e) => onChange({ ...value, newBranchName: e.target.value })}
          />
        </label>
      ) : (
        <div className="worktrees-form-modal__field-group">
          <CustomSelect
            id={branchSelectId}
            label={existingBranchLabel}
            value={value.selectedBranchName}
            options={branchOptions}
            disabled={pending || liveBranchesQuery.isLoading || branchOptions.length === 0}
            requirement="required"
            onChange={(next) => onChange({ ...value, selectedBranchName: next })}
          />
          {existingBranchHint ? (
            <p className="worktrees-form-modal__field-hint">{existingBranchHint}</p>
          ) : null}
        </div>
      )}
    </div>
  );
}

export function branchBindPayload(value: BranchBindValue): {
  name: string;
  create_branch?: boolean;
} | null {
  if (value.createNew) {
    const name = value.newBranchName.trim();
    return name !== "" ? { name, create_branch: true } : null;
  }
  if (value.selectedBranchName !== "") {
    return { name: value.selectedBranchName };
  }
  return null;
}

export type { BranchBindValue };
