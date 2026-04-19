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
    </div>
  );
}
