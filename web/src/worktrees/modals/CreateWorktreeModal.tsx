import { useState } from "react";
import { Modal } from "@/shared/Modal";
import { MutationErrorBanner } from "@/shared/MutationErrorBanner";
import { CustomSelect } from "@/components/custom-select";
import { WorkspaceDirPickerModal } from "@/components/workspace-picker";
import { gitDeleteErrorMessage } from "../gitDeleteErrors";
import { useGlobalLiveWorktrees } from "../hooks/useGlobalLiveWorktrees";
import { useAutoReconcileInventory } from "../hooks/useAutoReconcileInventory";
import { useLiveInventoryUnreachable } from "../hooks/useLiveInventoryUnreachable";
import { worktreeGitCopy } from "../worktreeGitCopy";
import { liveWorktreeOptionLabel } from "../worktreeGitCopy";
import {
  WorktreeBranchBindFields,
  branchBindPayload,
  type BranchBindValue,
} from "../components/WorktreeBranchBindFields";
import { WorktreeInventoryReconcilePrompt } from "../components/WorktreeInventoryReconcilePrompt";

type StartFromMode = "main" | "reference";

type Props = {
  open: boolean;
  pending: boolean;
  error: unknown;
  repositoryId: string;
  storedPath: string;
  reconcilePending?: boolean;
  reconcileError?: unknown;
  reconcileBlocked?: boolean;
  onReconcile: () => void;
  onClose: () => void;
  onSubmit: (input: {
    path: string;
    name?: string;
    branch: string;
    create_branch?: boolean;
    start_point?: string;
  }) => void;
};

export function CreateWorktreeModal({
  open,
  pending,
  error,
  repositoryId,
  storedPath,
  reconcilePending = false,
  reconcileError,
  reconcileBlocked = false,
  onReconcile,
  onClose,
  onSubmit,
}: Props) {
  const [path, setPath] = useState("");
  const [name, setName] = useState("");
  const [startFrom, setStartFrom] = useState<StartFromMode>("main");
  const [referencePath, setReferencePath] = useState("");
  const [branchBind, setBranchBind] = useState<BranchBindValue>({
    selectedBranchName: "",
    newBranchName: "",
    createNew: false,
  });
  const [pickerOpen, setPickerOpen] = useState(false);

  const liveWorktreesQuery = useGlobalLiveWorktrees(repositoryId, {
    enabled: open && repositoryId !== "",
  });
  const inventoryUnreachable = useLiveInventoryUnreachable(liveWorktreesQuery);
  useAutoReconcileInventory({
    enabled: open && repositoryId !== "",
    inventoryUnreachable,
    reconcilePending,
    reconcileBlocked,
    onReconcile,
  });
  const referenceOptions = (liveWorktreesQuery.data ?? [])
    .filter((wt) => wt.branch.trim() !== "")
    .map((wt) => ({
      value: wt.path,
      label: liveWorktreeOptionLabel(wt.path, wt.is_main),
    }));
  const referenceWorktree = liveWorktreesQuery.data?.find((wt) => wt.path === referencePath);
  const referenceDetached = startFrom === "reference" && referencePath !== "" && !referenceWorktree?.branch.trim();

  if (!open) return null;

  const errorMessage = error != null ? gitDeleteErrorMessage(error) : null;
  const branchPayload = branchBindPayload(branchBind);
  const referenceReady = startFrom === "main" || (referencePath !== "" && !referenceDetached);
  const canSubmit =
    !inventoryUnreachable && path.trim() !== "" && branchPayload != null && referenceReady;

  const startPoint =
    startFrom === "reference" && branchPayload?.create_branch && referenceWorktree?.branch.trim()
      ? referenceWorktree.branch.trim()
      : undefined;

  return (
    <>
      <Modal
        onClose={onClose}
        labelledBy="create-worktree-title"
        busy={pending}
        dismissibleWhileBusy={false}
      >
        <form
          className="panel modal-sheet worktrees-form-modal"
          onSubmit={(e) => {
            e.preventDefault();
            const trimmedPath = path.trim();
            if (!trimmedPath || !branchPayload) return;
            onSubmit({
              path: trimmedPath,
              name: name.trim() || undefined,
              branch: branchPayload.name,
              create_branch: branchPayload.create_branch,
              ...(startPoint ? { start_point: startPoint } : {}),
            });
          }}
        >
          <header className="worktrees-form-modal__header">
            <h2 id="create-worktree-title">{worktreeGitCopy.createModalTitle}</h2>
            <p className="worktrees-form-modal__lead">{worktreeGitCopy.createModalLead}</p>
          </header>
          {inventoryUnreachable ? (
            <WorktreeInventoryReconcilePrompt
              storedPath={storedPath}
              pending={reconcilePending}
              reconcileError={reconcileError}
              onReconcile={onReconcile}
            />
          ) : null}
          {!inventoryUnreachable ? (
            <>
          <fieldset className="worktrees-form-modal__fieldset">
            <legend className="settings-field-label">{worktreeGitCopy.createModalStartFromLabel}</legend>
            <label className="worktrees-form-modal__radio">
              <input
                type="radio"
                name="create-worktree-start-from"
                checked={startFrom === "main"}
                disabled={pending}
                onChange={() => setStartFrom("main")}
              />
              {worktreeGitCopy.createModalStartFromMain}
            </label>
            <label className="worktrees-form-modal__radio">
              <input
                type="radio"
                name="create-worktree-start-from"
                checked={startFrom === "reference"}
                disabled={pending}
                onChange={() => setStartFrom("reference")}
              />
              {worktreeGitCopy.createModalStartFromReference}
            </label>
          </fieldset>
          {startFrom === "reference" ? (
            <CustomSelect
              id="create-worktree-reference-select"
              label={worktreeGitCopy.createModalReferenceLabel}
              value={referencePath}
              options={referenceOptions}
              disabled={pending || liveWorktreesQuery.isLoading || referenceOptions.length === 0}
              requirement="required"
              onChange={setReferencePath}
            />
          ) : null}
          {referenceDetached ? (
            <p className="worktrees-form-modal__picker-empty" role="alert">
              {worktreeGitCopy.createModalReferenceDetached}
            </p>
          ) : null}
          <div className="worktrees-form-modal__picker">
            <p className="worktrees-form-modal__picker-label">{worktreeGitCopy.createModalPathLabel}</p>
            <button
              type="button"
              className="secondary"
              disabled={pending}
              onClick={() => setPickerOpen(true)}
            >
              {worktreeGitCopy.createModalChoosePath}
            </button>
            {path.trim() !== "" ? (
              <p className="worktrees-form-modal__selected">
                {worktreeGitCopy.createModalPathSelectedPrefix} <code>{path}</code>
              </p>
            ) : null}
          </div>
          <label className="field">
            <span className="settings-field-label">{worktreeGitCopy.createModalDisplayNameLabel}</span>
            <input
              type="text"
              value={name}
              disabled={pending}
              onChange={(e) => setName(e.target.value)}
              placeholder={worktreeGitCopy.createModalDisplayNamePlaceholder}
            />
          </label>
          <WorktreeBranchBindFields
            repositoryId={repositoryId}
            enabled={open && repositoryId !== ""}
            pending={pending}
            value={branchBind}
            onChange={setBranchBind}
            branchSelectId="create-worktree-branch-select"
            newBranchInputId="create-worktree-branch-new-name"
          />
            </>
          ) : null}
          {errorMessage ? (
            <MutationErrorBanner error={errorMessage} className="worktrees-form-modal__error" />
          ) : null}
          <div className="row stack-row-actions">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              {worktreeGitCopy.cancel}
            </button>
            <button
              type="submit"
              className="btn-primary"
              disabled={pending || !canSubmit}
            >
              {pending ? worktreeGitCopy.createModalSubmitting : worktreeGitCopy.createModalSubmit}
            </button>
          </div>
        </form>
      </Modal>
      <WorkspaceDirPickerModal
        open={pickerOpen}
        nested
        currentPath={path}
        onClose={() => setPickerOpen(false)}
        onSelect={(next) => {
          setPath(next);
          setPickerOpen(false);
        }}
      />
    </>
  );
}
