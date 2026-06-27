import { Modal } from "@/shared/Modal";
import type { GitLiveWorktree } from "@/types/git";
import {
  liveWorktreeOptionLabel,
  worktreeGitCopy,
} from "../worktreeGitCopy";
import "@/components/workspace-picker/workspace-picker.css";

type Props = {
  open: boolean;
  nested?: boolean;
  pending?: boolean;
  loading: boolean;
  worktrees: GitLiveWorktree[];
  selectedPath: string;
  onClose: () => void;
  onSelect: (path: string) => void;
};

export function LinkedWorktreePickerModal({
  open,
  nested = false,
  pending = false,
  loading,
  worktrees,
  selectedPath,
  onClose,
  onSelect,
}: Props) {
  if (!open) return null;

  const canConfirm = selectedPath.trim() !== "" && !pending;

  return (
    <Modal
      labelledBy="linked-worktree-picker-title"
      describedBy="linked-worktree-picker-lead"
      size="wide"
      stack={nested ? "nested" : "default"}
      lockBodyScroll={!nested}
      onClose={onClose}
    >
      <div className="panel modal-sheet workspace-picker-modal">
        <header className="workspace-picker-header">
          <h2 id="linked-worktree-picker-title" className="workspace-picker-title">
            {worktreeGitCopy.registerModalBrowseTitle}
          </h2>
          <p id="linked-worktree-picker-lead" className="workspace-picker-lead">
            {worktreeGitCopy.registerModalBrowseLead}
          </p>
        </header>

        {loading ? (
          <p className="workspace-picker-status">Loading linked worktrees…</p>
        ) : null}

        <ul className="workspace-picker-list" aria-busy={loading}>
          {!loading && worktrees.length === 0 ? (
            <li className="workspace-picker-empty">
              <p className="workspace-picker-empty-title">
                {worktreeGitCopy.registerModalBrowseEmptyTitle}
              </p>
              <p className="workspace-picker-empty-hint">
                {worktreeGitCopy.registerModalBrowseEmptyHint}
              </p>
            </li>
          ) : null}
          {!loading
            ? worktrees.map((wt) => {
                const label = liveWorktreeOptionLabel(wt.path, wt.is_main);
                const isSelected = wt.path === selectedPath;
                return (
                  <li key={wt.path}>
                    <button
                      type="button"
                      className="workspace-picker-row"
                      aria-pressed={isSelected}
                      disabled={pending}
                      onClick={() => onSelect(wt.path)}
                    >
                      <FolderIcon />
                      <span className="workspace-picker-row-main">
                        <span className="workspace-picker-row-name">{label}</span>
                        <span className="workspace-picker-row-sub">{wt.path}</span>
                      </span>
                      {wt.branch.trim() !== "" ? (
                        <span className="workspace-picker-git-badge">{wt.branch}</span>
                      ) : null}
                      <ChevronIcon />
                    </button>
                  </li>
                );
              })
            : null}
        </ul>

        <footer className="workspace-picker-footer">
          <div className="workspace-picker-selection" aria-live="polite">
            <span className="workspace-picker-selection-label">
              {worktreeGitCopy.registerModalPathLabel}
            </span>
            <code
              className="workspace-picker-selection-path"
              data-empty={selectedPath.trim() === ""}
            >
              {selectedPath.trim() !== ""
                ? selectedPath
                : worktreeGitCopy.registerModalBrowseSelectHint}
            </code>
          </div>
          <div className="workspace-picker-footer-actions">
            <button type="button" className="secondary" disabled={pending} onClick={onClose}>
              {worktreeGitCopy.cancel}
            </button>
            <button
              type="button"
              disabled={!canConfirm}
              onClick={() => {
                if (selectedPath.trim() === "") return;
                onSelect(selectedPath);
                onClose();
              }}
            >
              {worktreeGitCopy.registerModalBrowseConfirm}
            </button>
          </div>
        </footer>
      </div>
    </Modal>
  );
}

function FolderIcon() {
  return (
    <svg
      className="workspace-picker-row-icon"
      viewBox="0 0 20 20"
      width="18"
      height="18"
      aria-hidden="true"
    >
      <path
        d="M2.75 5.5A1.75 1.75 0 0 1 4.5 3.75h3.13c.46 0 .9.18 1.23.5l1.12 1.06H15.5c.97 0 1.75.78 1.75 1.75v7c0 .97-.78 1.75-1.75 1.75h-11A1.75 1.75 0 0 1 2.75 14V5.5Z"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function ChevronIcon() {
  return (
    <svg
      className="workspace-picker-row-chevron"
      viewBox="0 0 16 16"
      width="14"
      height="14"
      aria-hidden="true"
    >
      <path
        d="m6 4 4 4-4 4"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}
