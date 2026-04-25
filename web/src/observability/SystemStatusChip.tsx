import { useSystemHealth } from "./useSystemHealth";
import { summarize } from "./systemHealthViewModel";

type Props = {
  /**
   * Whether the SSE event stream is currently connected. Owned by
   * `useTasksApp`, so the chip stays a pure presentational composer
   * and does not reach into the SSE machinery itself.
   */
  connected: boolean;
};

/**
 * Compact header status chip composing two independent live signals
 * into one operator-facing surface:
 *
 *   - **Label** reflects the health summary (paused / degraded / ok /
 *     unknown), with SSE-aware tweaks: whenever the polled snapshot is
 *     missing (pending, failed, or unsettled) but the event stream is
 *     up, the headline reads **Connected** so it never contradicts the
 *     live dot / “Live updates” pill. **Status unavailable** is reserved
 *     for when both the snapshot is missing and SSE is down. Sourced
 *     from `useSystemHealth`, which polls
 *     every 10s; intentionally NOT SSE because /system/health is a
 *     pull endpoint (see docs/API-HTTP.md "System health" — publishing
 *     SSE here would loop forever).
 *
 *   - **Dot color** reflects the SSE connection (live = success-bright
 *     pulse, otherwise idle dot). This is the *how* — is the live
 *     update channel up? An SSE drop with healthy /system/health is a
 *     real problem (the SPA is now stale), and folding it into the
 *     same chip keeps the operator's attention on a single surface.
 *
 * Precedence: paused > degraded > ok > unknown (centralised in
 * `summarize`). The chip is **not** a link: it reads like live
 * connection/health state, so navigating on click felt like a trap.
 */
export function SystemStatusChip({ connected }: Props) {
  const { health, loading } = useSystemHealth();
  const summary = summarize(health, loading);

  const pillClass = pillClassForChip(summary.level, connected);
  const dotClass = connected ? "stream-dot stream-dot--live" : "stream-dot";
  const label = chipMainLabel(summary.level, connected, loading);
  const live = connected ? "Live updates" : "Reconnecting";
  const ariaLabel = `Status: ${label}. ${summary.caption}. Updates: ${live}.`;

  return (
    <div
      role="status"
      className="system-status-chip"
      aria-label={ariaLabel}
      title={`${label} — ${summary.caption}`}
      data-testid="system-status-chip"
      data-level={summary.level}
      data-sse={connected ? "live" : "down"}
    >
      <span className="system-status-chip-main">
        <span className={dotClass} aria-hidden />
        <span className="system-status-chip-text">{label}</span>
        <span className={`stream-pill ${pillClass}`}>{live}</span>
      </span>
    </div>
  );
}

function pillClassForChip(
  level: ReturnType<typeof summarize>["level"],
  connected: boolean,
): string {
  if (level === "unknown" && connected) {
    return "stream-pill--ok";
  }
  switch (level) {
    case "ok":
      return "stream-pill--ok";
    case "paused":
    case "degraded":
      return "stream-pill--warn";
    case "unknown":
    default:
      return "stream-pill--sync";
  }
}

function chipMainLabel(
  level: ReturnType<typeof summarize>["level"],
  connected: boolean,
  loading: boolean,
): string {
  switch (level) {
    case "ok":
      return "Healthy";
    case "paused":
      return "Agent paused";
    case "degraded":
      return "Degraded";
    case "unknown":
    default:
      if (loading) {
        return connected ? "Connected" : "Connecting…";
      }
      // Snapshot settled without data (e.g. fetch error). SSE may still
      // be live — don't imply the whole UI is "unavailable" when it is not.
      if (connected) {
        return "Connected";
      }
      return "Status unavailable";
  }
}
