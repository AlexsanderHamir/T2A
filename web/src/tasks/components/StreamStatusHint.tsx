type Props = {
  connected: boolean;
  /** Task list is refetching after SSE or focus (data may still be shown). */
  listSyncing?: boolean;
};

export function StreamStatusHint({ connected, listSyncing }: Props) {
  const showListBusy = Boolean(listSyncing);
  return (
    <div className="stream-status">
      <div className="stream-status-main">
        <span
          className={`stream-dot ${connected ? "stream-dot--live" : ""}`}
          aria-hidden
        />
        <span className="stream-status-text">
          {connected ? "Live updates" : "Waiting for connection"}
        </span>
        {showListBusy ? (
          <span
            className="stream-pill stream-pill--sync stream-pill--sync-enter"
            role="status"
          >
            {connected ? "Updating list…" : "Syncing…"}
          </span>
        ) : (
          <span
            className={`stream-pill ${connected ? "stream-pill--ok" : "stream-pill--warn"}`}
          >
            {connected ? "Connected" : "Disconnected"}
          </span>
        )}
      </div>
      <details className="stream-status-dev">
        <summary>Local development</summary>
        <p>
          Start <code>taskapi</code> on port <strong>8080</strong>, then{" "}
          <code>npm run dev</code> in <code>web/</code>. Events stream at{" "}
          <code>/events</code>. While connected, the header stays on{" "}
          <strong>Connected</strong> even if the list refetches in the background
          after each event. With <code>T2A_SSE_TEST=1</code> on the server,
          synthetic events fire about every <strong>3s</strong> by default (
          <code>T2A_SSE_TEST_INTERVAL</code>)—that refetch traffic is normal in
          dev; turn the test off or slow the interval if you do not need it.
        </p>
      </details>
    </div>
  );
}
