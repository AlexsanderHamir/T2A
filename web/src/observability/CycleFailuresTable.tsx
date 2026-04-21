import { useId, useLayoutEffect, useRef, useState } from "react";
import { Link } from "react-router-dom";
import type { TaskStatsRecentFailure } from "@/types/task";

/** When layout metrics are unavailable (e.g. tests), assume overflow past this length. */
const REASON_OVERFLOW_FALLBACK_CHARS = 160;

type Props = {
  failures: TaskStatsRecentFailure[];
};

/**
 * Shared table body for cycle_failed rows (Observability snapshot and the
 * full failures list page). Deep-links match RecentFailuresTable.
 */
export function CycleFailuresTable({ failures }: Props) {
  return (
    <table className="obs-failures-table">
      <thead>
        <tr>
          <th scope="col">When</th>
          <th scope="col">Task</th>
          <th scope="col">Attempt</th>
          <th scope="col">Status</th>
          <th scope="col">Reason</th>
        </tr>
      </thead>
      <tbody>
        {failures.map((f) => (
          <FailureRow key={`${f.task_id}-${f.event_seq}`} failure={f} />
        ))}
      </tbody>
    </table>
  );
}

function FailureRow({ failure }: { failure: TaskStatsRecentFailure }) {
  const eventHref = `/tasks/${failure.task_id}/events/${failure.event_seq}`;
  const taskHref = `/tasks/${failure.task_id}`;
  const statusClass =
    failure.status === "aborted"
      ? "cell-pill--status-blocked"
      : "cell-pill--status-failed";
  return (
    <tr data-testid={`obs-failure-row-${failure.task_id}-${failure.event_seq}`}>
      <td>
        <Link to={eventHref} className="obs-failures-link">
          <time dateTime={failure.at}>{formatTimestamp(failure.at)}</time>
        </Link>
      </td>
      <td>
        <Link to={taskHref} className="obs-failures-link obs-failures-link--mono">
          {shortId(failure.task_id)}
        </Link>
      </td>
      <td>#{failure.attempt_seq}</td>
      <td>
        <span className={`obs-failures-pill ${statusClass}`}>
          {failure.status}
        </span>
      </td>
      <td className="obs-failures-reason">
        <FailureReasonCell reason={failure.reason} />
      </td>
    </tr>
  );
}

function FailureReasonCell({ reason }: { reason: string }) {
  const trimmed = reason.trim();
  const textRef = useRef<HTMLParagraphElement>(null);
  const [expanded, setExpanded] = useState(false);
  const [clampedOverflow, setClampedOverflow] = useState(false);
  const textId = useId();

  useLayoutEffect(() => {
    const el = textRef.current;
    if (!el) return;

    const measure = () => {
      if (expanded) return;
      const sh = el.scrollHeight;
      const ch = el.clientHeight;
      const overflow =
        ch > 0 ? sh > ch + 1 : trimmed.length > REASON_OVERFLOW_FALLBACK_CHARS;
      setClampedOverflow(overflow);
    };

    measure();
    if (typeof ResizeObserver === "undefined") return undefined;
    const ro = new ResizeObserver(measure);
    ro.observe(el);
    return () => ro.disconnect();
  }, [expanded, trimmed]);

  if (!trimmed) {
    return <em className="obs-failures-muted">(no reason recorded)</em>;
  }

  const showToggle = clampedOverflow;
  const showMore = showToggle && !expanded;
  const showLess = showToggle && expanded;

  return (
    <div className="obs-failures-reason-cell">
      <p
        ref={textRef}
        id={textId}
        className={
          expanded
            ? "obs-failures-reason-text"
            : "obs-failures-reason-text obs-failures-reason-text--clamped"
        }
      >
        {trimmed}
      </p>
      {showMore ? (
        <button
          type="button"
          className="obs-failures-reason-toggle"
          aria-expanded={false}
          aria-controls={textId}
          onClick={() => setExpanded(true)}
        >
          Show more
        </button>
      ) : null}
      {showLess ? (
        <button
          type="button"
          className="obs-failures-reason-toggle"
          aria-expanded={true}
          aria-controls={textId}
          onClick={() => setExpanded(false)}
        >
          Show less
        </button>
      ) : null}
    </div>
  );
}

function formatTimestamp(iso: string): string {
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  return d.toLocaleString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
  });
}

function shortId(id: string): string {
  if (id.length <= 10) return id;
  return `${id.slice(0, 8)}…`;
}
