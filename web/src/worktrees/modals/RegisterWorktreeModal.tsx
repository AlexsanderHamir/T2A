import { useEffect, useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { CustomSelect } from "@/components/custom-select";
import { useGlobalLiveWorktrees } from "../hooks/useGlobalLiveWorktrees";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";
import {
  liveWorktreeOptionLabel,
  worktreeGitCopy,
} from "../worktreeGitCopy";
import {
  WorktreeBranchBindFields,
  branchBindPayload,
  type BranchBindValue,
} from "../components/WorktreeBranchBindFields";
import { LinkedWorktreePickerModal } from "./LinkedWorktreePickerModal";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  repositoryId: string;
  onClose: () => void;
  onSubmit: (input: {
    path: string;
    name?: string;
    branch?: { name: string; create_branch?: boolean };
  }) => void;
};

export function RegisterWorktreeModal({
  open,
  pending,
  error,
  repositoryId,
  onClose,
  onSubmit,
}: Props) {
  const [selectedPath, setSelectedPath] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [pickerOpen, setPickerOpen] = useState(false);
  const [pickerSelectedPath, setPickerSelectedPath] = useState("");
  const [branchBind, setBranchBind] = useState<BranchBindValue>({
    selectedBranchName: "",
    newBranchName: "",
    createNew: false,
  });

  const liveWorktreesQuery = useGlobalLiveWorktrees(repositoryId, {
    enabled: open && repositoryId !== "",
  });
  const liveWorktrees = (liveWorktreesQuery.data ?? []).filter((wt) => !wt.registered);
  const worktreeOptions = liveWorktrees.map((wt) => ({
    value: wt.path,
    label: liveWorktreeOptionLabel(wt.path, wt.is_main),
  }));

  useEffect(() => {
    if (!open) {
      setSelectedPath("");
      setDisplayName("");
      setPickerSelectedPath("");
      setBranchBind({ selectedBranchName: "", newBranchName: "", createNew: false });
    }
  }, [open]);

  useEffect(() => {
    if (!pickerOpen) return;
    setPickerSelectedPath(selectedPath);
  }, [pickerOpen, selectedPath]);

  useEffect(() => {
    if (!open || selectedPath === "" || branchBind.createNew) return;
    const match = liveWorktreesQuery.data?.find((wt) => wt.path === selectedPath);
    const branch = match?.branch.trim() ?? "";
    if (branch === "") return;
    setBranchBind((prev) =>
      prev.selectedBranchName === branch ? prev : { ...prev, selectedBranchName: branch },
    );
  }, [open, selectedPath, branchBind.createNew, liveWorktreesQuery.data]);

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;
  const branchPayload = branchBindPayload(branchBind);
  const canSubmit = selectedPath !== "" && branchPayload != null;

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="register-worktree-title"
        busy={pending}
        dismissibleWhileBusy={false}
      >
        <form
          className="panel modal-sheet worktrees-form-modal"
          onSubmit={(e) => {
            e.preventDefault();
            if (!canSubmit || !branchPayload) return;
            onSubmit({
              path: selectedPath,
              name: displayName.trim() || undefined,
              branch: branchPayload,
            });
          }}
        >
          <header className="worktrees-form-modal__header">
            <h2 id="register-worktree-title">{worktreeGitCopy.registerModalTitle}</h2>
            <p className="worktrees-form-modal__lead">{worktreeGitCopy.registerModalLead}</p>
          </header>

          <CustomSelect
            id="register-worktree-select"
            label={worktreeGitCopy.registerModalPathLabel}
            value={selectedPath}
            options={worktreeOptions}
            disabled={pending || liveWorktreesQuery.isLoading}
            requirement="required"
            onChange={setSelectedPath}
          />

          <div className="worktrees-form-modal__picker">
            <button
              type="button"
              className="secondary"
              disabled={pending}
              onClick={() => setPickerOpen(true)}
            >
              {worktreeGitCopy.registerModalBrowsePath}
            </button>
          </div>

          {selectedPath !== "" ? (
            <p className="worktrees-form-modal__selected">
              {worktreeGitCopy.registerModalPathSelectedPrefix}{" "}
              <code>{selectedPath}</code>
            </p>
          ) : null}

          {worktreeOptions.length === 0 && !liveWorktreesQuery.isLoading && selectedPath === "" ? (
            <p className="worktrees-form-modal__picker-empty">
              {worktreeGitCopy.registerModalPathEmpty}
            </p>
          ) : null}

          <label className="field">
            <span className="settings-field-label">{worktreeGitCopy.registerModalDisplayNameLabel}</span>
            <input
              type="text"
              value={displayName}
              disabled={pending}
              placeholder={worktreeGitCopy.registerModalDisplayNamePlaceholder}
              onChange={(e) => setDisplayName(e.target.value)}
            />
          </label>

          <WorktreeBranchBindFields
            repositoryId={repositoryId}
            enabled={open && repositoryId !== ""}
            pending={pending}
            value={branchBind}
            onChange={setBranchBind}
            branchSelectId="register-worktree-branch-select"
          />

          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}

          <div className="row stack-row-actions">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              {worktreeGitCopy.cancel}
            </button>
            <button type="submit" className="btn-primary" disabled={pending || !canSubmit}>
              {pending ? worktreeGitCopy.registerModalSubmitting : worktreeGitCopy.registerModalSubmit}
            </button>
          </div>
        </form>
      </Modal>
      <LinkedWorktreePickerModal
        open={pickerOpen}
        nested
        pending={pending}
        loading={liveWorktreesQuery.isLoading}
        worktrees={liveWorktrees}
        selectedPath={pickerSelectedPath}
        onClose={() => setPickerOpen(false)}
        onSelect={(next) => {
          setPickerSelectedPath(next);
          setSelectedPath(next);
        }}
      />
    </>
  );
}
