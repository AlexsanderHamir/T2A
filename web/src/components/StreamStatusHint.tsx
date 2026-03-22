type Props = {
  connected: boolean;
};

export function StreamStatusHint({ connected }: Props) {
  return (
    <p className="sub">
      Live updates via <code>/events</code>{" "}
      <span className={`badge ${connected ? "on" : "off"}`}>
        {connected ? "stream connected" : "stream disconnected"}
      </span>
      — start <code>taskapi</code> on port 8080, then <code>npm run dev</code>{" "}
      in <code>web/</code>.
    </p>
  );
}
