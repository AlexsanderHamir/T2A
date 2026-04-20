import type { KpiState } from "./kpiState";
import { useAnimatedNumber } from "./useAnimatedNumber";

type Props = {
  label: string;
  state: KpiState;
  /** One-line caption under the number (e.g. "needs investigation"). */
  meta: string;
  /** Optional accent: drives the left border accent color via CSS modifier. */
  tone?: "neutral" | "positive" | "warning" | "danger" | "info";
  testId?: string;
};

/**
 * Stat card with skeleton / unavailable / ready states. Mirrors the visual
 * language of `TaskHome` KPIs (border accent + value + caption) so the
 * Observability page does not look foreign to a user coming from Home.
 *
 * The ready-state number tweens via `useAnimatedNumber` so live
 * stats updates (SSE-driven stats refetch) glide rather than flash.
 * `aria-live="polite"` on the value lets assistive tech announce
 * the target, but only on settle — the transient in-between frames
 * are aria-hidden so the user doesn't hear every tween tick.
 */
export function KpiCard({ label, state, meta, tone = "neutral", testId }: Props) {
  const toneClass = `obs-kpi-card--${tone}`;
  return (
    <article
      className={`obs-kpi-card ${toneClass}`}
      aria-busy={state.kind === "loading"}
      data-testid={testId}
    >
      <p className="obs-kpi-label">{label}</p>
      {state.kind === "ready" ? (
        <KpiValue value={state.value} />
      ) : state.kind === "loading" ? (
        <p className="obs-kpi-value" aria-hidden="true">
          <span className="skeleton-block skeleton-block--kpi-value" />
          <span className="visually-hidden">Loading {label}</span>
        </p>
      ) : (
        <p
          className="obs-kpi-value obs-kpi-value--unavailable"
          aria-label={`${label} unavailable`}
        >
          —
        </p>
      )}
      <p className="obs-kpi-meta">{meta}</p>
    </article>
  );
}

function KpiValue({ value }: { value: number }) {
  const animated = useAnimatedNumber(value);
  return (
    <p className="obs-kpi-value" aria-live="polite">
      <span aria-hidden="true">{animated}</span>
      <span className="visually-hidden">{value}</span>
    </p>
  );
}
