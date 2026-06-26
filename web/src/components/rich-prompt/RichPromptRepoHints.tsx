type Props = {
  showSelectWorktreeHint: boolean;
  showRepoMisconfigHint: boolean;
  workspaceBroken: boolean;
  fileSearchFailedWhileAvailable: boolean;
  showRepoUnknownHint: boolean;
  showFileSearchSpinner: boolean;
};

/** Status copy under the rich prompt when repo / @ search is misconfigured or busy. */
export function RichPromptRepoHints({
  showSelectWorktreeHint,
  showRepoMisconfigHint,
  workspaceBroken,
  fileSearchFailedWhileAvailable,
  showRepoUnknownHint,
  showFileSearchSpinner,
}: Props) {
  return (
    <>
      {showSelectWorktreeHint ? (
        <p className="mention-repo-hint" role="status">
          Select a worktree above to enable <code>@file</code> mentions.
        </p>
      ) : null}
      {showRepoMisconfigHint ? (
        <p className="mention-repo-hint" role="status">
          {workspaceBroken ? (
            <>
              The selected worktree path is missing or not a directory.
              Update it on the{" "}
              <a href="/worktrees" target="_blank" rel="noopener noreferrer">
                Worktrees page
              </a>{" "}
              to restore <code>@file</code> mentions.
            </>
          ) : fileSearchFailedWhileAvailable ? (
            <>
              File search failed for the selected worktree. Check the worktree
              path on the{" "}
              <a href="/worktrees" target="_blank" rel="noopener noreferrer">
                Worktrees page
              </a>{" "}
              or inspect the server logs.
            </>
          ) : (
            <>
              File search is not available for the selected worktree. Check the
              worktree on the{" "}
              <a href="/worktrees" target="_blank" rel="noopener noreferrer">
                Worktrees page
              </a>{" "}
              to enable <code>@file</code> mentions.
            </>
          )}
        </p>
      ) : null}
      {showRepoUnknownHint ? (
        <p className="mention-repo-hint" role="status">
          Could not verify file search for the selected worktree. Check the
          worktree on the{" "}
          <a href="/worktrees" target="_blank" rel="noopener noreferrer">
            Worktrees page
          </a>
          .
        </p>
      ) : null}
      {showFileSearchSpinner ? (
        <p
          className="mention-repo-hint mention-repo-hint--pending"
          role="status"
          aria-live="polite"
        >
          Searching files…
        </p>
      ) : null}
    </>
  );
}
