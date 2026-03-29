type Props = {
  connected: boolean;
  /** Task list is refetching after SSE or focus (data may still be shown). */
  listSyncing?: boolean;
};

export function StreamStatusHint({ connected, listSyncing }: Props) {
  return (
    <p className="sub">
      Live updates via <code>/events</code>{" "}
      <span className={`badge ${connected ? "on" : "off"}`}>
        {connected ? "stream connected" : "stream disconnected"}
      </span>
      {listSyncing ? (
        <>
          {" "}
          <span className="badge sync">list syncing</span>
        </>
      ) : null}
      — start <code>taskapi</code> on port 8080, then <code>npm run dev</code>{" "}
      in <code>web/</code>.
    </p>
  );
}
