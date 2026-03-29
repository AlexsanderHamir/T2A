import { useDelayedTrue } from "@/lib/useDelayedTrue";

type Props = {
  connected: boolean;
  /** Task list is refetching after SSE or focus (data may still be shown). */
  listSyncing?: boolean;
  /** When true (default), the syncing pill waits briefly before appearing. */
  smoothTransitions?: boolean;
};

const SYNC_PILL_DELAY_MS = 200;

export function StreamStatusHint({
  connected,
  listSyncing,
  smoothTransitions = true,
}: Props) {
  const syncDelayMs = smoothTransitions ? SYNC_PILL_DELAY_MS : 0;
  const showSyncPill = useDelayedTrue(Boolean(listSyncing), syncDelayMs);

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
        {listSyncing && showSyncPill ? (
          <span
            className="stream-pill stream-pill--sync stream-pill--sync-enter"
            role="status"
          >
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
