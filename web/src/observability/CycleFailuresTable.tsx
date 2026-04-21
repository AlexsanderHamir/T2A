import { useId, useState } from "react";
import { Link } from "react-router-dom";
import type { TaskStatsRecentFailure } from "@/types/task";

/** Collapsed preview length (long API reasons stay one compact block until expanded). */
const FAILURE_REASON_PREVIEW_CHARS = 120;

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

function truncateFailureReasonPreview(text: string, maxLen: number): string {
  if (text.length <= maxLen) return text;
  const slice = text.slice(0, maxLen);
  const lastSpace = slice.lastIndexOf(" ");
  if (lastSpace > 0 && lastSpace >= maxLen * 0.45) {
    return `${slice.slice(0, lastSpace)}…`;
  }
  return `${slice.trimEnd()}…`;
}

function FailureReasonCell({ reason }: { reason: string }) {
  const [expanded, setExpanded] = useState(false);
  const baseId = useId();
  const textId = `${baseId}-text`;
  const needsMore =
    reason.trim().length > FAILURE_REASON_PREVIEW_CHARS;

  if (!reason.trim()) {
    return <em className="obs-failures-muted">(no reason recorded)</em>;
  }

  const preview = truncateFailureReasonPreview(
    reason,
    FAILURE_REASON_PREVIEW_CHARS,
  );
  const showFull = !needsMore || expanded;

  return (
    <div className="obs-failures-reason-cell">
      <span id={textId} className="obs-failures-reason-text">
        {showFull ? reason : preview}
      </span>
      {needsMore ? (
        <button
          type="button"
          className="obs-failures-reason-toggle"
          onClick={() => setExpanded((v) => !v)}
          aria-expanded={expanded}
          aria-controls={textId}
        >
          {expanded ? "Show less" : "Read more"}
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
