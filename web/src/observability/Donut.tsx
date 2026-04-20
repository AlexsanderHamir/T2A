type Slice = {
  id: string;
  label: string;
  value: number;
  /** CSS class name applied to the SVG arc (uses design-token colors). */
  fillClass: string;
};

type Props = {
  title: string;
  slices: Slice[];
  /** Caption rendered below the donut (e.g. "12 tasks"). */
  caption?: string;
};

/**
 * Two- or three-slice donut chart used for the parent / subtask scope split.
 * Renders an SVG <circle> per slice using `stroke-dasharray` so we don't
 * pull in a charting library for a single visualization. Empty data shows
 * a neutral ring so the layout never reflows when the first stat lands.
 *
 * Geometry: the ring is a single 80px-diameter circle drawn as 360° of
 * stroke; each slice owns a fraction of that circumference proportional
 * to its value. `strokeDashoffset` rotates each arc to the cumulative
 * sweep of the previous slices, so the slices butt against each other
 * with no manual angle math beyond the running sum.
 */
const RADIUS = 36;
const CIRCUMFERENCE = 2 * Math.PI * RADIUS;

export function Donut({ title, slices, caption }: Props) {
  const total = slices.reduce((acc, s) => acc + Math.max(0, s.value), 0);
  const hasData = total > 0;
  let cumulative = 0;
  return (
    <section className="obs-donut" aria-label={title}>
      <header className="obs-bar-head">
        <h3 className="obs-bar-title">{title}</h3>
        {caption ? <p className="obs-bar-caption">{caption}</p> : null}
      </header>
      <div className="obs-donut-frame">
        <svg
          className="obs-donut-svg"
          viewBox="0 0 100 100"
          role="img"
          aria-label={hasData ? donutAriaLabel(title, slices, total) : `${title}: no data yet`}
        >
          <circle
            className="obs-donut-track"
            cx="50"
            cy="50"
            r={RADIUS}
            fill="transparent"
          />
          {hasData
            ? slices.map((s) => {
                const safe = Math.max(0, s.value);
                if (safe === 0) return null;
                const length = (safe / total) * CIRCUMFERENCE;
                const offset = -cumulative;
                cumulative += length;
                return (
                  <circle
                    key={s.id}
                    className={`obs-donut-arc ${s.fillClass}`}
                    cx="50"
                    cy="50"
                    r={RADIUS}
                    fill="transparent"
                    strokeDasharray={`${length} ${CIRCUMFERENCE - length}`}
                    strokeDashoffset={offset}
                    data-testid={`obs-donut-arc-${s.id}`}
                  >
                    <title>{`${s.label}: ${safe}`}</title>
                  </circle>
                );
              })
            : null}
        </svg>
        <ul className="obs-donut-legend" aria-label={`${title} legend`}>
          {slices.map((s) => (
            <li key={s.id} className="obs-donut-legend-item">
              <span
                className={`obs-donut-legend-swatch ${s.fillClass}`}
                aria-hidden="true"
              />
              <span className="obs-donut-legend-label">{s.label}</span>
              <span className="obs-donut-legend-value">{s.value}</span>
            </li>
          ))}
        </ul>
      </div>
    </section>
  );
}

function donutAriaLabel(title: string, slices: Slice[], total: number): string {
  const parts = slices
    .filter((s) => s.value > 0)
    .map((s) => {
      const pct = Math.round((s.value / total) * 100);
      return `${s.label} ${s.value} (${pct}%)`;
    });
  return `${title}: ${parts.join(", ")}`;
}
