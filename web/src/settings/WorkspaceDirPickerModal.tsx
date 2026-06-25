import { Fragment, useCallback, useEffect, useMemo, useState } from "react";
import { Modal } from "@/shared/Modal";
import {
  browseWorkspaceDirs,
  fetchWorkspaceRoots,
  type BrowseDirEntry,
  type WorkspaceBrowseRoot,
} from "@/api/settingsBrowse";
import "./workspace-picker.css";

type Props = {
  open: boolean;
  onClose: () => void;
  onSelect: (path: string) => void;
  currentPath: string;
  /** Opens above another modal (worktrees register flow). */
  nested?: boolean;
  title?: string;
  lead?: string;
  /** When true, only git checkouts can be confirmed; folders stay visible. */
  requireGitRepository?: boolean;
};

type LoadState =
  | { kind: "idle" }
  | { kind: "loading" }
  | { kind: "ready"; roots: WorkspaceBrowseRoot[]; environment: "native" | "docker" }
  | { kind: "error"; message: string };

type Crumb = { label: string; path: string };

const USER_FOLDER_CATEGORIES = new Set([
  "documents",
  "desktop",
  "downloads",
  "pictures",
  "music",
  "videos",
]);

function partitionBrowseRoots(roots: WorkspaceBrowseRoot[]): {
  workspace: WorkspaceBrowseRoot[];
  userFolders: WorkspaceBrowseRoot[];
} {
  const workspace: WorkspaceBrowseRoot[] = [];
  const userFolders: WorkspaceBrowseRoot[] = [];
  for (const root of roots) {
    if (root.category && USER_FOLDER_CATEGORIES.has(root.category)) {
      userFolders.push(root);
    } else {
      workspace.push(root);
    }
  }
  return { workspace, userFolders };
}

// Splits the current path into breadcrumb segments anchored to whichever
// starting location contains it. Without anchoring to a root, deep paths
// like /Users/x/code/proj would surface every system folder as a crumb,
// which is noise — what matters is "Home > code > proj".
function computeCrumbs(
  roots: WorkspaceBrowseRoot[],
  currentBrowsePath: string,
): Crumb[] {
  const trimmed = currentBrowsePath.trim();
  if (trimmed === "") return [];
  const root = roots.find(
    (r) =>
      trimmed === r.path ||
      trimmed.startsWith(`${r.path}/`) ||
      trimmed.startsWith(`${r.path}\\`),
  );
  if (!root) {
    return [{ label: trimmed, path: trimmed }];
  }
  const crumbs: Crumb[] = [{ label: root.label, path: root.path }];
  if (trimmed === root.path) return crumbs;
  const sep = trimmed.includes("\\") ? "\\" : "/";
  const rel = trimmed.slice(root.path.length).replace(/^[\\/]+/, "");
  const parts = rel.split(/[\\/]+/).filter(Boolean);
  let acc = root.path;
  for (const part of parts) {
    acc = `${acc}${sep}${part}`;
    crumbs.push({ label: part, path: acc });
  }
  return crumbs;
}

export function WorkspaceDirPickerModal({
  open,
  onClose,
  onSelect,
  nested = false,
  title = "Choose folder",
  lead,
  requireGitRepository = false,
}: Props) {
  const resolvedLead =
    lead ??
    (requireGitRepository
      ? "Navigate to a git repository checkout. Hamix needs a .git folder at the path you register."
      : "Open a folder to browse inside it. Confirm the folder you’re in to register it.");
  const [loadState, setLoadState] = useState<LoadState>({ kind: "idle" });
  const [entries, setEntries] = useState<BrowseDirEntry[]>([]);
  const [currentBrowsePath, setCurrentBrowsePath] = useState("");
  const [parentPath, setParentPath] = useState("");
  const [currentPathIsGitRepo, setCurrentPathIsGitRepo] = useState(false);
  const [listingError, setListingError] = useState<string | null>(null);
  const [listingPending, setListingPending] = useState(false);

  const atRoots = currentBrowsePath.trim() === "";

  const loadListing = useCallback(async (path: string) => {
    setListingPending(true);
    setListingError(null);
    try {
      const listing = await browseWorkspaceDirs(path);
      setEntries(listing.entries);
      setCurrentBrowsePath(listing.path ?? path);
      setParentPath(listing.parent_path ?? "");
      setCurrentPathIsGitRepo(listing.is_git_repo === true);
    } catch (err) {
      setListingError(err instanceof Error ? err.message : "Could not list folders");
      setEntries([]);
    } finally {
      setListingPending(false);
    }
  }, []);

  useEffect(() => {
    if (!open) return;
    let cancelled = false;
    setLoadState({ kind: "loading" });
    setEntries([]);
    setCurrentBrowsePath("");
    setParentPath("");
    setCurrentPathIsGitRepo(false);
    setListingError(null);
    void fetchWorkspaceRoots()
      .then((roots) => {
        if (cancelled) return;
        setLoadState({
          kind: "ready",
          roots: roots.roots,
          environment: roots.environment,
        });
      })
      .catch((err) => {
        if (cancelled) return;
        setLoadState({
          kind: "error",
          message: err instanceof Error ? err.message : "Could not load locations",
        });
      });
    return () => {
      cancelled = true;
    };
  }, [open]);

  const crumbs = useMemo(() => {
    if (loadState.kind !== "ready") return [];
    return computeCrumbs(loadState.roots, currentBrowsePath);
  }, [loadState, currentBrowsePath]);

  function goRoots() {
    setEntries([]);
    setCurrentBrowsePath("");
    setParentPath("");
    setCurrentPathIsGitRepo(false);
    setListingError(null);
  }

  function goBack() {
    if (atRoots || listingPending) return;
    if (parentPath.trim() === "") {
      goRoots();
      return;
    }
    void loadListing(parentPath);
  }

  function confirmSelection() {
    if (atRoots || listingPending || currentBrowsePath.trim() === "") return;
    if (requireGitRepository && !currentPathIsGitRepo) return;
    onSelect(currentBrowsePath);
    onClose();
  }

  const hasOpenFolder = !atRoots && currentBrowsePath.trim() !== "";
  const gitRequirementMet = !requireGitRepository || currentPathIsGitRepo;
  const canConfirm = hasOpenFolder && !listingPending && gitRequirementMet;

  const rootGroups =
    loadState.kind === "ready" ? partitionBrowseRoots(loadState.roots) : null;

  if (!open) return null;

  return (
    <Modal
      labelledBy="workspace-dir-picker-title"
      describedBy="workspace-dir-picker-lead"
      size="wide"
      stack={nested ? "nested" : "default"}
      lockBodyScroll={!nested}
      onClose={onClose}
    >
      <div className="panel modal-sheet workspace-picker-modal">
        <header className="workspace-picker-header">
          <h2 id="workspace-dir-picker-title" className="workspace-picker-title">
            {title}
          </h2>
          <p id="workspace-dir-picker-lead" className="workspace-picker-lead">
            {resolvedLead}
          </p>
        </header>

        {loadState.kind === "loading" ? (
          <p className="workspace-picker-status">Loading locations…</p>
        ) : null}

        {loadState.kind === "error" ? (
          <p
            className="workspace-picker-status workspace-picker-status--error"
            role="alert"
          >
            {loadState.message}
          </p>
        ) : null}

        {loadState.kind === "ready" ? (
          <>
            {loadState.environment === "docker" ? (
              <p className="workspace-picker-hint">
                Folders shown here live inside the dev container. Your home directory is
                mounted at <code>/host-home</code>.
              </p>
            ) : null}

            {atRoots ? (
              <p className="workspace-picker-section-label">
                Choose a folder to browse from
              </p>
            ) : (
              <PickerBreadcrumb
                crumbs={crumbs}
                listingPending={listingPending}
                onBack={goBack}
                onJump={(path) => void loadListing(path)}
              />
            )}

            {listingError ? (
              <p
                className="workspace-picker-status workspace-picker-status--error"
                role="alert"
              >
                {listingError}
              </p>
            ) : null}

            <ul className="workspace-picker-list" aria-busy={listingPending}>
              {atRoots && rootGroups ? (
                <>
                  {rootGroups.workspace.length > 0 ? (
                    <RootGroup
                      label="Workspace"
                      roots={rootGroups.workspace}
                      listingPending={listingPending}
                      onOpen={(path) => void loadListing(path)}
                    />
                  ) : null}
                  {rootGroups.userFolders.length > 0 ? (
                    <RootGroup
                      label="User folders"
                      roots={rootGroups.userFolders}
                      listingPending={listingPending}
                      onOpen={(path) => void loadListing(path)}
                    />
                  ) : null}
                </>
              ) : null}
              {!atRoots
                ? entries.map((entry) => (
                    <li key={entry.path}>
                      <FolderRow
                        name={entry.name}
                        gitRepoStatus={
                          requireGitRepository
                            ? entry.is_git_repo
                            : entry.is_git_repo
                              ? true
                              : undefined
                        }
                        disabled={listingPending}
                        onClick={() => void loadListing(entry.path)}
                      />
                    </li>
                  ))
                : null}
              {!atRoots && !listingPending && entries.length === 0 ? (
                <li className="workspace-picker-empty">
                  <p className="workspace-picker-empty-title">
                    No subfolders inside this folder.
                  </p>
                  <p className="workspace-picker-empty-hint">
                    {requireGitRepository && !currentPathIsGitRepo
                      ? "This folder is not a git checkout. Go back and open a folder that contains a .git directory."
                      : requireGitRepository
                        ? "Use the button below to register this git checkout, or go back to pick a different folder."
                        : "Use the button below to register this folder, or go back to pick a different one."}
                  </p>
                </li>
              ) : null}
            </ul>

            <footer className="workspace-picker-footer">
              <div className="workspace-picker-selection" aria-live="polite">
                <span className="workspace-picker-selection-label">
                  {requireGitRepository ? "Repository checkout" : "Folder to register"}
                </span>
                <code
                  className="workspace-picker-selection-path"
                  data-empty={!hasOpenFolder}
                >
                  {hasOpenFolder ? currentBrowsePath : "Open a folder to register it"}
                </code>
              </div>
              <div className="workspace-picker-footer-actions">
                <button type="button" className="secondary" onClick={onClose}>
                  Cancel
                </button>
                <button
                  type="button"
                  disabled={!canConfirm}
                  onClick={confirmSelection}
                >
                  Use this folder
                </button>
              </div>
            </footer>
          </>
        ) : null}
      </div>
    </Modal>
  );
}

type RootGroupProps = {
  label: string;
  roots: WorkspaceBrowseRoot[];
  listingPending: boolean;
  onOpen: (path: string) => void;
};

function RootGroup({ label, roots, listingPending, onOpen }: RootGroupProps) {
  return (
    <li className="workspace-picker-root-group">
      <p className="workspace-picker-section-label">{label}</p>
      <ul className="workspace-picker-root-group-list">
        {roots.map((root) => (
          <li key={root.id}>
            <FolderRow
              name={root.label}
              sublabel={root.path}
              disabled={listingPending || !root.available}
              onClick={() => onOpen(root.path)}
            />
            {!root.available && root.unavailable_reason ? (
              <p className="workspace-picker-row-note">{root.unavailable_reason}</p>
            ) : null}
          </li>
        ))}
      </ul>
    </li>
  );
}

type PickerBreadcrumbProps = {
  crumbs: Crumb[];
  listingPending: boolean;
  onBack: () => void;
  onJump: (path: string) => void;
};

function PickerBreadcrumb({
  crumbs,
  listingPending,
  onBack,
  onJump,
}: PickerBreadcrumbProps) {
  return (
    <nav className="workspace-picker-crumbs" aria-label="Folder location">
      <button
        type="button"
        className="workspace-picker-back"
        onClick={onBack}
        disabled={listingPending}
        aria-label="Go up one folder"
      >
        <BackIcon />
        <span>Back</span>
      </button>
      <ol className="workspace-picker-crumb-path">
        {crumbs.map((crumb, idx) => {
          const isLast = idx === crumbs.length - 1;
          return (
            <Fragment key={crumb.path}>
              {idx > 0 ? (
                <li aria-hidden="true" className="workspace-picker-crumb-sep">
                  /
                </li>
              ) : null}
              <li>
                <button
                  type="button"
                  className="workspace-picker-crumb"
                  onClick={() => onJump(crumb.path)}
                  disabled={isLast || listingPending}
                  aria-current={isLast ? "location" : undefined}
                  title={crumb.path}
                >
                  {crumb.label}
                </button>
              </li>
            </Fragment>
          );
        })}
      </ol>
    </nav>
  );
}

type FolderRowProps = {
  name: string;
  sublabel?: string;
  /** When set, shows a git-status icon before the chevron. */
  gitRepoStatus?: boolean;
  disabled?: boolean;
  onClick: () => void;
};

function FolderRow({ name, sublabel, gitRepoStatus, disabled, onClick }: FolderRowProps) {
  return (
    <button
      type="button"
      className="workspace-picker-row"
      onClick={onClick}
      disabled={disabled}
    >
      <FolderIcon />
      <span className="workspace-picker-row-main">
        <span className="workspace-picker-row-name">{name}</span>
        {sublabel ? (
          <span className="workspace-picker-row-sub">{sublabel}</span>
        ) : null}
      </span>
      {gitRepoStatus !== undefined ? (
        <GitRepoStatusIcon isGitRepo={gitRepoStatus} />
      ) : null}
      <ChevronIcon />
    </button>
  );
}

function GitRepoStatusIcon({ isGitRepo }: { isGitRepo: boolean }) {
  return (
    <span
      className={
        isGitRepo
          ? "workspace-picker-git-icon workspace-picker-git-icon--yes"
          : "workspace-picker-git-icon workspace-picker-git-icon--no"
      }
      title={isGitRepo ? "Git repository" : "Not a git repository"}
      aria-label={isGitRepo ? "Git repository" : "Not a git repository"}
    >
      {isGitRepo ? <GitBranchIcon /> : <NoGitRepoIcon />}
    </span>
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

function BackIcon() {
  return (
    <svg viewBox="0 0 16 16" width="13" height="13" aria-hidden="true">
      <path
        d="M10 3 5 8l5 5"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function GitBranchIcon() {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
      <path
        d="M4.5 3.25a1.25 1.25 0 1 0 0 2.5 1.25 1.25 0 0 0 0-2.5ZM4.5 10.25a1.25 1.25 0 1 0 0 2.5 1.25 1.25 0 0 0 0-2.5ZM11.5 6.75a1.25 1.25 0 1 0 0 2.5 1.25 1.25 0 0 0 0-2.5Z"
        fill="currentColor"
      />
      <path
        d="M5.75 5.5v3.25M5.75 5.5h3.5a1.25 1.25 0 0 1 1.25 1.25v1.25"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.25"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function NoGitRepoIcon() {
  return (
    <svg viewBox="0 0 16 16" width="16" height="16" aria-hidden="true">
      <circle
        cx="8"
        cy="8"
        r="5.75"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.25"
      />
      <path
        d="M5.25 5.25 10.75 10.75"
        fill="none"
        stroke="currentColor"
        strokeWidth="1.25"
        strokeLinecap="round"
      />
    </svg>
  );
}
