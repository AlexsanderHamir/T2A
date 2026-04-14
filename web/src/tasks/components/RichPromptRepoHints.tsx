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
              The workspace folder for <code>REPO_ROOT</code> is missing or not a
              directory on the machine running <code>taskapi</code>. Fix the path
              and restart <code>taskapi</code>.
            </>
          ) : fileSearchFailedWhileAvailable ? (
            <>
              File search failed even though the server reports a workspace.
              Restart <code>taskapi</code> or check server logs.
            </>
          ) : (
            <>
              No repository is configured for file search. Set{" "}
              <code>REPO_ROOT</code> in the server environment (same{" "}
              <code>.env</code> as <code>DATABASE_URL</code>) and restart{" "}
              <code>taskapi</code> from the repo root so it loads that{" "}
              <code>.env</code>.
            </>
          )}
        </p>
      ) : null}
      {showRepoUnknownHint ? (
        <p className="mention-repo-hint" role="status">
          Could not verify workspace file search. For local dev, run{" "}
          <code>taskapi</code> and the Vite dev server so <code>/health</code>{" "}
          and <code>/repo</code> proxy to the API (see <code>web/vite.config</code>
          ).
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
