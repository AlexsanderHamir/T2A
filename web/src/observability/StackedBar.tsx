type Segment = {
  /** Stable identity used for keys, tooltips, and color tokens. */
  id: string;
  /** Visible label inside the legend (and the segment if it fits). */
  label: string;
  value: number;
  /**
   * Either a CSS class on the colored fill (e.g. `cell-pill--status-failed`)
   * or a CSS color string. Class is preferred so the chart inherits the same
   * status / priority palette as the task list.
   */
  fillClass?: string;
};

type Props = {
  segments: Segment[];
  /** Accessible name; rendered visually as the section header. */
  title: string;
  /**
   * Optional caption rendered under the bar (e.g. "12 total"). Empty bars
   * fall back to a documented placeholder so the section never collapses
   * to nothing on an empty database.
   */
  caption?: string;
};

/**
 * Horizontal stacked bar with an inline legend. Segment widths are computed
 * from the sum of values (zero-total falls back to a neutral placeholder so
 * the row keeps its height and screen readers still get a meaningful
 * accessible name). Each segment exposes a tooltip with the raw count plus
 * percentage so hovering tells the operator the exact slice without
 * opening dev tools.
 *
 * Click-through is intentionally NOT wired in this stage: the home-page
 * filter is component-state, not URL-state, so a `?status=` deep link
 * would be a dead affordance today. We'll add it once the list filter
 * learns to read the URL (tracked in the observability rollout plan).
 */
export function StackedBar({ segments, title, caption }: Props) {
  const total = segments.reduce((acc, s) => acc + Math.max(0, s.value), 0);
  const hasData = total > 0;
  return (
    <section className="obs-bar" aria-label={title}>
      <header className="obs-bar-head">
        <h3 className="obs-bar-title">{title}</h3>
        {caption ? <p className="obs-bar-caption">{caption}</p> : null}
      </header>
      <div
        className="obs-bar-track"
        role="img"
        aria-label={
          hasData
            ? segmentsAriaLabel(title, segments, total)
            : `${title}: no data yet`
        }
      >
        {hasData ? (
          segments.map((s) => {
            const safe = Math.max(0, s.value);
            if (safe === 0) return null;
            const pct = (safe / total) * 100;
            const className = ["obs-bar-segment", s.fillClass ?? ""]
              .filter(Boolean)
              .join(" ");
            return (
              <span
                key={s.id}
                className={className}
                style={{ width: `${pct}%` }}
                title={`${s.label}: ${safe} (${formatPct(pct)})`}
                data-testid={`obs-bar-segment-${s.id}`}
              />
            );
          })
        ) : (
          <span className="obs-bar-empty" aria-hidden="true" />
        )}
      </div>
      <ul className="obs-bar-legend" aria-label={`${title} legend`}>
        {segments.map((s) => (
          <li key={s.id} className="obs-bar-legend-item">
            <span
              className={`obs-bar-legend-swatch ${s.fillClass ?? ""}`}
              aria-hidden="true"
            />
            <span className="obs-bar-legend-label">{s.label}</span>
            <span className="obs-bar-legend-value">{s.value}</span>
          </li>
        ))}
      </ul>
    </section>
  );
}

function segmentsAriaLabel(
  title: string,
  segments: Segment[],
  total: number,
): string {
  const parts = segments
    .filter((s) => s.value > 0)
    .map((s) => {
      const pct = formatPct((s.value / total) * 100);
      return `${s.label} ${s.value} (${pct})`;
    });
  return `${title}: ${parts.join(", ")}`;
}

function formatPct(pct: number): string {
  if (!Number.isFinite(pct) || pct <= 0) return "0%";
  if (pct >= 99.5) return "100%";
  if (pct < 1) return "<1%";
  return `${Math.round(pct)}%`;
}
