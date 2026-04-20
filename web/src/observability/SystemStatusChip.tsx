import { Link } from "react-router-dom";
import { useSystemHealth } from "./useSystemHealth";
import { summarize } from "./systemHealthViewModel";

type Props = {
  /**
   * Whether the SSE event stream is currently connected. Owned by
   * `useTasksApp` (the same value previously fed to StreamStatusHint),
   * so the chip stays a pure presentational composer — it does not
   * reach into the SSE machinery itself.
   */
  connected: boolean;
};

/**
 * Compact header status chip composing two independent live signals
 * into one operator-facing surface:
 *
 *   - **Label** reflects the system-health summary (paused / degraded
 *     / ok / unknown). This is the *what* — what is the system
 *     currently doing? Sourced from `useSystemHealth`, which polls
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
 * `summarize`). The chip is a Link to /observability so a click takes
 * the operator straight to the pane that explains the current label.
 */
export function SystemStatusChip({ connected }: Props) {
  const { health, loading } = useSystemHealth();
  const summary = summarize(health, loading);

  const pillClass = pillClassForLevel(summary.level);
  const dotClass = connected ? "stream-dot stream-dot--live" : "stream-dot";
  const label = labelForLevel(summary.level);
  const live = connected ? "Live updates" : "Reconnecting";
  const ariaLabel = `System status: ${label}. ${summary.caption}. SSE: ${live}.`;

  return (
    <Link
      to="/observability"
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
    </Link>
  );
}

function pillClassForLevel(level: ReturnType<typeof summarize>["level"]): string {
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

function labelForLevel(level: ReturnType<typeof summarize>["level"]): string {
  switch (level) {
    case "ok":
      return "System OK";
    case "paused":
      return "Agent paused";
    case "degraded":
      return "Degraded";
    case "unknown":
    default:
      return "System unknown";
  }
}
