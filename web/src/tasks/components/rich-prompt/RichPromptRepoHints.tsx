type Props = {
  showRepoMisconfigHint: boolean;
  workspaceBroken: boolean;
  fileSearchFailedWhileAvailable: boolean;
  showRepoUnknownHint: boolean;
  showFileSearchSpinner: boolean;
};

/** Status copy under the rich prompt when repo / @ search is misconfigured or busy. */
export function RichPromptRepoHints({
  showRepoMisconfigHint,
  workspaceBroken,
  fileSearchFailedWhileAvailable,
  showRepoUnknownHint,
  showFileSearchSpinner,
}: Props) {
  return (
    <>
      {showRepoMisconfigHint ? (
        <p className="mention-repo-hint" role="status">
          {workspaceBroken ? (
            <>
              The configured repository root is missing or not a directory.
              Update it on the <a href="/settings">Settings page</a> to restore{" "}
              <code>@file</code> mentions.
            </>
          ) : fileSearchFailedWhileAvailable ? (
            <>
              File search failed even though the server reports a workspace.
              Check the repository root on the{" "}
              <a href="/settings">Settings page</a> or inspect the server
              logs.
            </>
          ) : (
            <>
              No repository is configured for file search. Set the{" "}
              <strong>Repository root</strong> on the{" "}
              <a href="/settings">Settings page</a> to enable{" "}
              <code>@file</code> mentions.
            </>
          )}
        </p>
      ) : null}
      {showRepoUnknownHint ? (
        <p className="mention-repo-hint" role="status">
          Could not verify workspace file search. Check the repository root on
          the <a href="/settings">Settings page</a>.
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
