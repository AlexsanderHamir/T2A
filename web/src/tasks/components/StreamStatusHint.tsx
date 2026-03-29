type Props = {
  connected: boolean;
  /** Task list is refetching after SSE or focus (data may still be shown). */
  listSyncing?: boolean;
};

export function StreamStatusHint({ connected, listSyncing }: Props) {
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
        {listSyncing ? (
          <span className="stream-pill stream-pill--sync" role="status">
            Syncing…
          </span>
        ) : null}
        <span
          className={`stream-pill ${connected ? "stream-pill--ok" : "stream-pill--warn"}`}
        >
          {connected ? "Connected" : "Disconnected"}
        </span>
      </div>
      <details className="stream-status-dev">
        <summary>Local development</summary>
        <p>
          Start <code>taskapi</code> on port <strong>8080</strong>, then{" "}
          <code>npm run dev</code> in <code>web/</code>. Events stream at{" "}
          <code>/events</code>.
        </p>
      </details>
    </div>
  );
}
