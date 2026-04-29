/**
 * Compact relative-time formatting for operator-facing timestamps.
 *
 * Used by surfaces that need to read like Stripe / Linear / Apple
 * settings pages — e.g. "Edited 5 min ago", "Edited 3 h ago",
 * "Edited 2 d ago" — instead of dumping a raw ISO timestamp into
 * the row.
 *
 * Buckets:
 *  - < 45s            → "just now"
 *  - < 60 min         → "<n> min ago"
 *  - < 24 h           → "<n> h ago"
 *  - < 7 d            → "<n> d ago"
 *  - < 30 d           → "<n> w ago"
 *  - < 365 d          → "<n> mo ago"
 *  - else             → "<n> y ago"
 *
 * Future timestamps (e.g. clock skew) collapse to "just now" so the
 * UI never reads "in 3 min" for a draft the operator saved a moment
 * ago. We trade exactness here for a more reassuring affordance.
 *
 * Returns "" when the input is empty / not parseable so callers can
 * conditionally render: `time ? <span>{time}</span> : null`.
 */
export function formatRelativeTime(
  iso: string | null | undefined,
  now: Date = new Date(),
): string {
  if (!iso) return "";
  const then = new Date(iso);
  const t = then.getTime();
  if (!Number.isFinite(t)) return "";

  const deltaMs = now.getTime() - t;

  if (deltaMs < 45_000) return "just now";

  const minutes = Math.floor(deltaMs / 60_000);
  if (minutes < 60) return `${minutes} min ago`;

  const hours = Math.floor(deltaMs / 3_600_000);
  if (hours < 24) return `${hours} h ago`;

  const days = Math.floor(deltaMs / 86_400_000);
  if (days < 7) return `${days} d ago`;

  const weeks = Math.floor(days / 7);
  if (days < 30) return `${weeks} w ago`;

  const months = Math.floor(days / 30);
  if (days < 365) return `${months} mo ago`;

  const years = Math.floor(days / 365);
  return `${years} y ago`;
}
