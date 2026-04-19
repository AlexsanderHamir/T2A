import { CopyableId } from "@/shared/CopyableId";

const UUID_RE =
  /^[0-9a-f]{8}-[0-9a-f]{4}-[1-5][0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/i;

/** Turn snake_case keys into short title-case labels for the overview list. */
export function humanizeEventDataKey(key: string): string {
  return key
    .split("_")
    .filter(Boolean)
    .map((w) => w.charAt(0).toUpperCase() + w.slice(1).toLowerCase())
    .join(" ");
}

function looksLikeUuid(s: string): boolean {
  return s.length >= 32 && UUID_RE.test(s);
}

function ValueCell({ value }: { value: unknown }) {
  if (value === null || value === undefined) {
    return <span className="muted">—</span>;
  }
  if (typeof value === "boolean") {
    return <span>{value ? "true" : "false"}</span>;
  }
  if (typeof value === "number") {
    return <span>{Number.isFinite(value) ? String(value) : "—"}</span>;
  }
  if (typeof value === "string") {
    if (looksLikeUuid(value)) {
      return <CopyableId value={value} />;
    }
    return <span className="task-event-generic-str">{value}</span>;
  }
  if (Array.isArray(value) || typeof value === "object") {
    return (
      <pre className="task-event-generic-nested-pre">
        {JSON.stringify(value, null, 2)}
      </pre>
    );
  }
  return <span className="muted">—</span>;
}

/**
 * Readable key/value layout for any event `data` object (fallback when there is
 * no specialized phase overview).
 */
export function GenericEventDataOverview({
  data,
}: {
  data: Record<string, unknown>;
}) {
  const keys = Object.keys(data).sort((a, b) => a.localeCompare(b));

  if (keys.length === 0) {
    return (
      <div className="task-event-generic-overview task-event-generic-overview--empty">
        <p className="muted">No payload fields for this event.</p>
      </div>
    );
  }

  return (
    <div className="task-event-generic-overview">
      <dl className="task-event-generic-dl">
        {keys.map((key) => (
          <div key={key}>
            <dt>{humanizeEventDataKey(key)}</dt>
            <dd>
              <ValueCell value={data[key]} />
            </dd>
          </div>
        ))}
      </dl>
    </div>
  );
}
